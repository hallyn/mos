package mosconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/project-machine/trust/pkg/trust"
	imagesource "stackerbuild.io/stacker/pkg/types"
)

// An ImageType can be either an ISO or a Zap layer.
type ImageType string

const (
	ISO ImageType = "iso"
	ZAP ImageType = "zap"
)

// Update can be full, meaning all existing Targets are replaced, or
// partial, meaning those in the install manifest are installed or
// replaced, but any other Targets on the system remain.

// An install manifest is a shipped, signed manifest of Targets.
// A system manifest is an intermediary list of targets actually
// installed on the system.  In a full install, the system manifest
// will contain the full set of targets in the install manifest.  On
// a partial install, the system manifest contains the new targets as
// well as any pre-existing targets which the new install manifest
// did not replace.

type UpdateType string

const (
	PartialUpdate UpdateType = "partial"
	FullUpdate    UpdateType = "complete"
)

func ParseUpdateType(t string) (UpdateType, error) {
	switch t {
	case "partial":
		return PartialUpdate, nil
	case "complete":
		return FullUpdate, nil
	default:
		return "", fmt.Errorf("Unknown update type")
	}
}

const CurrentInstallFileVersion = 1

// Only host network supported right now.
// To do: simple/nat, CNI
type TargetNetworkType string

const (
	HostNetwork TargetNetworkType = "host"
	NoNetwork   TargetNetworkType = "none"
)

type TargetNetwork struct {
	Type TargetNetworkType `json:"type"`
}

type ServiceType string

const (
	HostfsService    ServiceType = "hostfs"
	ContainerService ServiceType = "container"
	FsService        ServiceType = "fs-only"
)

type Target struct {
	ServiceName  string        `json:"service_name"` // name of target
	ImagePath    string        `json:"imagepath"`    // full image repository path
	Version      string        `json:"version"`      // docker or oci version tag
	ServiceType  ServiceType   `json:"service_type"`
	Network      TargetNetwork `json:"network"`
	NSGroup      string        `json:"nsgroup"`
	Digest       string        `json:"digest"`
	Size         int64         `json:"size"`
}
type InstallTargets []Target

func (t *Target) NeedsIdmap() bool {
	return t.NSGroup != "" && t.NSGroup != "none"
}

// This describes an install manifest
type InstallFile struct {
	Version     int            `json:"version"`
	ImageType   ImageType      `json:"image_type"`
	Product     string         `json:"product"`
	Targets     InstallTargets `json:"targets"`
	UpdateType  UpdateType     `json:"update_type"`
	StorageType StorageType    `json:"storage_type"`
}

// Note we only do combined uid+gid ranges, range 65536, and only starting at
// container id 0.
type IdmapSet struct {
	Name   string `json:"idmap-name"` // This is the NSGroup specified in target
	Hostid int64  `json:"hostid"`
}

// SysTarget exists as an intermediary between a 'system manifest'
// and an 'install manifest'
type SysTarget struct {
	Name   string `json:"name"`   // the name of the target
	Source string `json:"source"` // the content address manifest file defining it

	raw         *Target
	OCIManifest ispec.Manifest
	OCIConfig   ispec.Image
}
type SysTargets []SysTarget

func (s *SysTargets) Contains(needle SysTarget) (SysTarget, bool) {
	for _, t := range *s {
		if t.Name == needle.Name {
			return t, true
		}
	}
	return SysTarget{}, false
}

type SysManifest struct {
	UidMaps    []IdmapSet  `json:"uidmaps"`
	SysTargets []SysTarget `json:"targets"`
}

func (af *InstallFile) Validate() error {
	if af.Product == "" {
		return fmt.Errorf("Must specify a product")
	}

	if af.Version > CurrentInstallFileVersion || af.Version < 1 {
		return fmt.Errorf("unsupported atomix file version: %d", af.Version)
	}

	if err := af.Targets.Validate(); err != nil {
		return err
	}

	if af.UpdateType == "" {
		af.UpdateType = PartialUpdate
	}

	return nil
}

func simpleParseInstall(manifestPath string) (InstallFile, error) {
	bytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return InstallFile{}, errors.Wrapf(err, "Failed reading manifest")
	}
	var manifest InstallFile
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return InstallFile{}, errors.Wrapf(err, "Failed parsing manifest")
	}

	return manifest, nil
}

// Verify an install.json manifest.  Return the parsed manifest.
// @is is the InstallSource of the install.json.
// @s is the storage driver, currently always an atomfs.
func ReadVerifyInstallManifest(is InstallSource, capath string, s Storage) (InstallFile, error) {
	bytes, err := os.ReadFile(is.FilePath)
	if err != nil {
		return InstallFile{}, fmt.Errorf("Failed reading manifest: %w", err)
	}

	if err := trust.VerifyManifest(bytes, is.SignPath, is.CertPath, capath); err != nil {
		return InstallFile{}, err
	}

	var manifest InstallFile
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return InstallFile{}, fmt.Errorf("Failed parsing manifest: %w", err)
	}

	// We've verified the install.json contents.  Now verify that the container
	// image manifest files pointed to have not been altered.
	for _, t := range manifest.Targets {
		if is.ocirepo != nil {
			// Import the layer into our zot store.
			// We could consider deleting the layer if VerifyTarget fails below.
			// This is not terribly important as nothing will use it,
			// unless there's a manifest which is properly signed which refers
			// to it, in which case we'll regret having deleted it...
			src := fmt.Sprintf("docker://%s/%s:%s", is.ocirepo.addr, t.ImagePath, t.Version)
			if err := s.ImportTarget(src, &t); err != nil {
				return InstallFile{}, err
			}
		}

		if err := s.VerifyTarget(&t); err != nil {
			return InstallFile{}, fmt.Errorf("Bad manifest hash for %q: %w", t.ServiceName, err)
		}
	}

	if err := manifest.Validate(); err != nil {
		return InstallFile{}, err
	}

	return manifest, nil
}

func (ts InstallTargets) Validate() error {
	for _, t := range ts {
		if t.ServiceName == "" {
			return fmt.Errorf("Target field 'name' cannot be empty: %#v", t)
		}

		if t.Version == "" {
			return fmt.Errorf("Target %s cannot have empty version", t.ServiceName)
		}

		if !t.ValidateNetwork() {
			return fmt.Errorf("Target %s has bad network: %#v", t.ServiceName, t.Network)
		}
	}

	return nil
}

// From a list of targets provided by the user, build an install.json.
// Most of the fields are not set here, but need to be set by the caller.
// The function simply
//   1. unmarshalls the input file
//   2. replace the ImagePath from one which is used to import to one one which
//      will be used on the host.
func ManifestFromTargets(infile string) (InstallFile, InstallTargets, error) {
	bad1 := InstallFile{}
	bad2 := InstallTargets{}

	bytes, err := os.ReadFile(infile)
	if err != nil {
		return bad1, bad2, fmt.Errorf("Failed reading %q: %w", infile, err)
	}

	manifest := InstallFile{}
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return bad1, bad2, fmt.Errorf("Failed unmarshaling input file: %w", err)
	}

	inTargets := InstallTargets{}
	for key, t := range manifest.Targets {
		inTargets = append(inTargets, t)
		b, err := calculateImagePath(t.ImagePath)
		if err != nil {
			return bad1, bad2, fmt.Errorf("Failed parsing image path %q: %w", t.ImagePath, err)
		}
		manifest.Targets[key].ImagePath = b
	}

	return manifest, inTargets, nil
}

func calculateImagePath(url string) (string, error) {
	src, err := imagesource.NewImageSource(url)
	if err != nil {
		return "", err
	}

	// For oci:oci:image:version, src.Url will be oci:image:version, and we
	// want to return 'image'.
	// For docker://zothub.io/machine/baseos:1.0.2 src.Url will be
	// zothub.io/machine/baseos:1.0.2, and we want to return machine/baseos

	switch src.Type {
	case "oci":
		tag, err := src.ParseTag()
		if err != nil {
			return "", err
		}
		r := strings.SplitN(tag, ":", 2)
		return r[0], nil
	case "docker":
		du, err := imagesource.NewDockerishUrl(url)
		if err != nil {
			return "", err
		}
		tag := strings.TrimPrefix(du.Path, "/")
		r := strings.SplitN(tag, ":", 2)
		return r[0], nil
	default:
		return "", fmt.Errorf("Unhandled image type %s", src.Type)
	}
}
