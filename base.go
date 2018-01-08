package stacker

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/layer"
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
	default:
		return fmt.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func getDocker(o BaseLayerOpts) error {
	tag, err := o.Layer.From.ParseTag()
	if err != nil {
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
		fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, tag),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("skopeo copy: %s", err)
	}

	target := path.Join(o.Config.RootFSDir, o.Target)
	fmt.Println("unpacking to", target)
	err = o.OCI.Unpack(tag, target, &layer.MapOptions{})
	if err != nil {
		return err
	}

	// umoci generates the config.json from the image, and also adds some
	// umoci metadata in a .umoci directory; we don't want to snapshot
	// either of these things, so let's remove them both.
	err = os.Remove(path.Join(target, "config.json"))
	if err != nil {
		return err
	}

	rootfs := path.Join(target, "rootfs")
	ents, err := ioutil.ReadDir(rootfs)
	if err != nil {
		return err
	}

	for _, e := range ents {
		if strings.Contains(e.Name(), "umoci") {
			if err := os.Remove(e.Name()); err != nil {
				return err
			}
		}
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

	layerPath := path.Join(o.Config.RootFSDir, o.Target, "rootfs")
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		return err
	}

	output, err := exec.Command("tar", "xf", tar, "-C", layerPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %s: %s", err, string(output))
	}

	return nil
}
