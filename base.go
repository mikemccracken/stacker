package stacker

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/openSUSE/umoci"
)

type BaseLayerOpts struct {
	Config StackerConfig
	Name   string
	Target string
	Layer  *Layer
	Cache  *BuildCache
	OCI    *umoci.Layout
}

func GetBaseLayer(o BaseLayerOpts) error {
	switch o.Layer.From.Type {
	case BuiltType:
		/* nothing to do assuming layers are imported in dependency order */
		return nil
	case TarType:
		return getTar(o)
	case OCIType:
		return fmt.Errorf("not implemented")
	case DockerType:
		return getDocker(o)
	case ScratchType:
		return getScratch(o)
	default:
		return fmt.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func getDocker(o BaseLayerOpts) error {
	tag, err := o.Layer.From.ParseTag()
	if err != nil {
		return err
	}

	// Note that we can do tihs over the top of the cache every time, since
	// skopeo should be smart enough to only copy layers that have changed.
	// Perhaps we want to do an `umoci gc` at some point, but for now we
	// don't bother.
	cacheDir := path.Join(o.Config.StackerDir, "layer-bases", tag)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cmd := exec.Command(
		"skopeo",
		// So we don't have to make everyone install an
		// /etc/containers/policy.json too. Alternatively, we could
		// write a default policy out to /tmp and use --policy.
		"--insecure-policy",
		"copy",
		o.Layer.From.Url,
		fmt.Sprintf("oci:%s:%s", cacheDir, tag),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("skopeo copy: %s", err)
	}

	// We just copied it to the cache, now let's copy that over to our image.
	cmd = exec.Command(
		"skopeo",
		"--insecure-policy",
		"copy",
		fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, tag),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy from cache to ocidir: %s: %s", err, string(output))
	}

	target := path.Join(o.Config.RootFSDir, o.Target)
	fmt.Println("unpacking to", target)

	image := fmt.Sprintf("%s:%s", o.Config.OCIDir, tag)
	cmd = exec.Command(
		"umoci",
		"unpack",
		"--image",
		image,
		target)

	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error during unpack: %s: %s", err, string(output))
	}

	return nil
}

func umociInit(o BaseLayerOpts) error {
	cmd := exec.Command(
		"umoci",
		"new",
		"--image",
		fmt.Sprintf("%s:%s", o.Config.OCIDir, o.Name))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umoci layout creation failed: %s: %s", err, string(output))
	}

	cmd = exec.Command(
		"umoci",
		"unpack",
		"--image",
		fmt.Sprintf("%s:%s", o.Config.OCIDir, o.Name),
		path.Join(o.Config.RootFSDir, ".working"))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umoci empty unpack failed: %s: %s", err, string(output))
	}

	layerPath := path.Join(o.Config.RootFSDir, o.Target, "rootfs")
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		return err
	}

	return nil
}

func getTar(o BaseLayerOpts) error {
	cacheDir := path.Join(o.Config.StackerDir, "layer-bases")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	tar, err := download(cacheDir, o.Layer.From.Url)
	if err != nil {
		return err
	}

	err = umociInit(o)
	if err != nil {
		return err
	}

	// TODO: make this respect ID maps
	layerPath := path.Join(o.Config.RootFSDir, o.Target, "rootfs")
	output, err := exec.Command("tar", "xf", tar, "-C", layerPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %s: %s", err, string(output))
	}

	return nil
}

func getScratch(o BaseLayerOpts) error {
	return umociInit(o)
}
