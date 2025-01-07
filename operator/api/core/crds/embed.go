package crds

import _ "embed"

var (
	//go:embed grove.io_podcliques.yaml
	podCliqueCRD string
	//go:embed grove.io_podgangsets.yaml
	podGangSetCRD string
)

// PodCliqueCRD returns the embedded PodClique CRD
func PodCliqueCRD() string {
	return podCliqueCRD
}

// PodGangSetCRD returns the embedded PodGangSet CRD
func PodGangSetCRD() string {
	return podGangSetCRD
}
