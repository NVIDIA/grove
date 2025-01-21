package crds

import _ "embed"

var (
	//go:embed grove.io_podgangs.yaml
	podGangCRD string
)

// PodGangCRD returns the PodGang CRD
func PodGangCRD() string {
	return podGangCRD
}
