load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "rejects credential-like substitute values" {
    bad_stacker build --substitute GH_TOKEN=ghp_pdwvRxGPzScphRuygVf863401f6269efda | \
        grep "refusing substitution \"GH_TOKEN=ghp_pdwvRxGPzScphRuygVf863401f6269efda\""
}
