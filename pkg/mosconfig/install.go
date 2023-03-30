package mosconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/umoci"
	"github.com/pkg/errors"
	"github.com/project-machine/trust/pkg/trust"
	"github.com/urfave/cli"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v2"
	"stackerbuild.io/stacker/pkg/lib"
)

type ociurl struct {
	name    string // the oci name (<name> in distribution spec)- e.g. foo/image
	tag     string // the oci tag, e.g. 1.0.0
	mDigest string // the digest for this image's manifest
	mSize   int64
	fDigest string // the digest for this file's contents - the actual blob digest
}

func (r *ocirepo) pickUrl(base string) (ociurl, error) {
	url := ociurl{}
	base = dropURLPrefix(base)
	s := strings.SplitN(base, "/", 2)
	if len(s) != 2 {
		return url, errors.Errorf("Failed parsing oci repo url: no '/'in %q", base)
	}
	s = strings.SplitN(s[1], ":", 2)
	if len(s) != 2 {
		return url, errors.Errorf("Failed parsing oci repo url: no ':'in %q", base)
	}
	url.name = s[0]
	url.tag = s[1]

	// Get the image digests we need from,  e.g.
	// http://0.0.0.0:18080/v2/machine/install/manifests/1.0.0
	u := fmt.Sprintf("http://%s/v2/%s/manifests/%s", r.addr, url.name, url.tag)
	resp, err := http.Get(u)
	if err != nil {
		return url, errors.Wrapf(err, "Failed connecting to %q", u)
	}
	if resp.StatusCode != 200 {
		return url, errors.Errorf("Bad status code connecting to %q: %d", u, resp.StatusCode)
	}
	defer resp.Body.Close()

	// This is the digest we need to use to get the list of referrers
	url.mDigest = resp.Header.Get("Docker-Content-Digest")
	if url.mDigest == "" {
		return url, errors.Errorf("No Docker-Content-Digest received")
	}
	strSize := resp.Header.Get("Content-Length")
	url.mSize, err = strconv.ParseInt(strSize, 10, 64)

	// Read the actual ispec.Index and get the Digest for Layer 1 - that
	// is the actual digest of the blob we want
	manifest := ispec.Manifest{}
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		    return url, errors.Wrapf(err, "Failed parsing the install artifact manifest")
	}
	if len(manifest.Layers) == 0 {
		return url, errors.Errorf("No layers found in the install artifact manifest!")
	}
	if len(manifest.Layers) > 1 {
		return url, errors.Errorf("More than one layer found in the install artifact manifest.")
	}

	url.fDigest = manifest.Layers[0].Digest.String()

	return url, nil
}

type ocirepo struct {
	addr string // 10.0.2.2:5000 or /mnt/oci
}

func dropURLPrefix(url string) string {
	prefixes := []string{"docker://", "http://", "https://"}
	for _, p := range prefixes {
		if strings.HasPrefix(url, p) {
			url = url[len(p):]
		}
	}
	return url
}

// Given a 10.0.2.2:5000/foo/install.json, set addr to
// http://10.0.2.2:5000, and check for connection using
// http://10.0.2.2:5000/v2
func NewOciRepo(base string) (*ocirepo, error) {
	r := ocirepo{}
	base = dropURLPrefix(base)
	s := strings.SplitN(base, "/", 2)
	if len(s) != 2 {
		return &r, errors.Errorf("Failed parsing oci repo url: no '/' in %q", base)
	}
	base = s[0]

	url := "http://" + base + "/v2/"
	resp, err := http.Get(url)
	if err != nil {
		return &r, errors.Errorf("Failed connecting to %q", url)
	}
	if resp.StatusCode != 200 {
		return &r, errors.Errorf("Bad status code %d connecting to %q: %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()

	r.addr = base
	return &r, nil
}

func (r *ocirepo) FetchFile(path string, dest string) error {
	url := "http://" + r.addr + "/v2/" + path
	resp, err := http.Get(url)
	if err != nil {
		return errors.Errorf("Failed connecting to %q", url)
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("Bad status code connecting to %q: %d", url, resp.StatusCode)
	}
	source := resp.Body
	defer source.Close()

	outf, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer outf.Close()

	_, err = io.Copy(outf, source)
	if err != nil {
		return err
	}

	return nil
}

func (r *ocirepo) FetchInstall(url ociurl, dest string) error {
	u := url.name + "/blobs/" + url.fDigest
	return r.FetchFile(u, dest)
}

const (
	pubkeyArtifact = "vnd.machine.pubkeycrt"
	sigArtifact = "vnd.machine.signature"
)

// end-12b     GET     /v2/<name>/referrers/<digest>?artifactType=<artifactType>     200     404/400
func (r *ocirepo) GetReferrers(url ociurl, artifactType string) (ispec.Index, error) {
	// Now fetch /v2/<name>/referrers/<digest>?artifactType=<artifactType>
	idx := ispec.Index{}
	u := fmt.Sprintf("http://%s/v2/%s/referrers/%s?artifactType=%s", r.addr, url.name, url.mDigest, artifactType)
	resp, err := http.Get(u)
	if err != nil {
		return idx, errors.Errorf("Failed connecting to %q", u)
	}
	if resp.StatusCode != 200 {
		return idx, errors.Errorf("Bad status code connecting to %q: %d", u, resp.StatusCode)
	}
	body := resp.Body
	defer body.Close()

	err = json.NewDecoder(body).Decode(&idx)
	if err != nil {
		    return idx, errors.Wrapf(err, "Failed parsing the list of referrers")
	}

	if len(idx.Manifests) == 0 {
		return idx, errors.Errorf("No manifest for artifact type %v at %#v (queried url %q)", artifactType, url, u)
	}

	return idx, nil
}

func (r *ocirepo) fetchArtifact(url ociurl, artifactType, dest string) error {
	referrer, err := r.GetReferrers(url, artifactType)
	if err != nil {
		return errors.Wrapf(err, "Failed getting list of referrers")
	}

	if len(referrer.Manifests) > 1 {
		// What do do?  Should we find one right here that the capath can verify?
		// Probably - but for now, just take the first one.
		fmt.Println("Warning: multiple referrers found, using first one")
	}

	digest := referrer.Manifests[0].Digest

	// we have the digest for a manifest whose layers[0] contains
	// the artifact we're looking for
	u := fmt.Sprintf("http://%s/v2/%s/blobs/%s", r.addr, url.name, digest)
	manifest := ispec.Manifest{}

	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("bad response code from oci repo")
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		return err
	}
	if len(manifest.Layers) == 0 {
		return errors.Errorf("Error parsing artifacts list")
	}

	digest = manifest.Layers[0].Digest
	u = fmt.Sprintf("%s/blobs/%s", url.name, digest)
	return r.FetchFile(u, dest)
}

func (r *ocirepo) FetchCert(url ociurl, dest string) error {
	return r.fetchArtifact(url, pubkeyArtifact, dest)
}

func (r *ocirepo) FetchSignature(url ociurl, dest string) error {
	return r.fetchArtifact(url, sigArtifact, dest)
}

// InstallSource represents an install file, its signature, and
// certificate for verifying the signature, downloaded from an
// oci repo url and its referrers.
type InstallSource struct {
	Basedir  string
	FilePath string
	CertPath string
	SignPath string
	ocirepo  *ocirepo

	NeedsCleanup bool
}

// cleaning up is only done if we created the tempdir
func (is *InstallSource) Cleanup() {
	if is.NeedsCleanup {
		os.RemoveAll(is.Basedir)
		is.NeedsCleanup = false
	}
}

func (is *InstallSource) FetchFromZot(inUrl string) error {
	dir, err := os.MkdirTemp("", "install")
	if err != nil {
		return err
	}
	is.Basedir = dir
	is.FilePath = filepath.Join(is.Basedir, "install.json") // TODO - switch to json
	is.CertPath = filepath.Join(is.Basedir, "manifestCert.pem")
	is.SignPath = filepath.Join(is.Basedir, "install.json.signed")

	r, err := NewOciRepo(inUrl)
	if err != nil {
		return errors.Wrapf(err, "Error opening OCI repo connection")
	}
	is.ocirepo = r

	url, err := r.pickUrl(inUrl)
	if err != nil {
		return errors.Wrapf(err, "Error parsing install manifest url")
	}

	err = r.FetchInstall(url, is.FilePath)
	if err != nil {
		return errors.Wrapf(err, "Error fetching the install manifest")
	}

	err = r.FetchCert(url, is.CertPath)
	if err != nil {
		return errors.Wrapf(err, "Error fetching the certificate")
	}

	err = r.FetchSignature(url, is.SignPath)
	if err != nil {
		return errors.Wrapf(err, "Error fetching the signature")
	}

	is.NeedsCleanup = true

	return nil
}

func InitializeMos(ctx *cli.Context) error {
	// Expect config, scratch-writes, and atomfs-store to exist
	storeDir := ctx.String("atomfs-store")
	if !PathExists(storeDir) {
		return errors.Errorf("atomfs store not found")
	}

	configDir := ctx.String("config-dir")
	if !PathExists(configDir) {
		return errors.Errorf("mos config directory not found")
	}

	args := ctx.Args()
	if len(args) < 1 {
		return errors.Errorf("An install source is required.\nUsage: mos install [--config-dir /config] [--atomfs-store /atomfs-store] docker://10.0.2.2:5000/mos/install.json:1.0")
	}

	var is InstallSource
	defer is.Cleanup()

	err := is.FetchFromZot(args[0])
	if err != nil {
		return err
	}

	caPath := "/manifestCA.pem"
	if ctx.IsSet("ca-path") {
		caPath = ctx.String("ca-path")
	}
	if !PathExists(caPath) {
		return errors.Errorf("Install manifest CA missing")
	}

	mos, err := NewMos(configDir, storeDir)
	if err != nil {
		return errors.Errorf("Error opening manifest: %w", err)
	}
	defer mos.Close()

	// Well, bit of a chicken and egg problem here.  We parse the configfile
	// first so we can copy all the needed zot images.
	cf, err := simpleParseInstall(is.FilePath)
	if err != nil {
		return errors.Wrapf(err, "Failed parsing install configuration")
	}

	for _, target := range cf.Targets {
		src := fmt.Sprintf("docker://%s/mos:%s", is.ocirepo.addr, dropHashAlg(target.Digest))
		err = mos.storage.ImportTarget(src, &target)
		if err != nil {
			return errors.Wrapf(err, "Failed reading targets while initializing mos")
		}
	}

	if cf.UpdateType == PartialUpdate {
		return errors.Errorf("Cannot install with a partial manifest")
	}

	// Finally set up our manifest store
	// The manifest will be re-read as it is verified.
	err = mos.initManifest(is.FilePath, is.CertPath, caPath, configDir)
	if err != nil {
		return errors.Errorf("Error initializing system manifest: %w", err)
	}

	return nil
}

func PublishManifest(ctx *cli.Context) error {
	cert := ctx.String("cert")
	if cert == "" {
		return fmt.Errorf("Certificate filename is required")
	}
	key := ctx.String("key")
	if key == "" {
		return fmt.Errorf("Key filename is required")
	}
	repo := ctx.String("repo")
	if cert == "" {
		return fmt.Errorf("Repo is required")
	}
	destpath := ctx.String("dest-path")
	if cert == "" {
		return fmt.Errorf("Repo is required")
	}
	args := ctx.Args()
	if len(args) != 1 {
		return fmt.Errorf("file is a required positional argument")
	}
	infile := args[0]

	bytes, err := os.ReadFile(infile)
	if err != nil {
		return errors.Wrapf(err, "Error reading %s", infile)
	}

	var imports ImportFile
	err = yaml.Unmarshal(bytes, &imports)
	if err != nil {
		return errors.Wrapf(err, "Error parsing %s", infile)
	}

	if imports.Version != CurrentInstallFileVersion {
		return errors.Errorf("Unknown import file version: %d (I know about %d)", imports.Version, CurrentInstallFileVersion)
	}

	install := InstallFile{
		Version: imports.Version,
		Product: imports.Product,
		UpdateType: imports.UpdateType,
	}

	// Copy each of the targets to specified oci repo,
	// verify digest and size, and append them to the install
	// manifest's list.
	for _, t := range imports.Targets {
		digest, size, err := getSizeDigest(t.Source)
		if err != nil {
			return errors.Wrapf(err, "Failed checking %s", t.Source)
		}
		if t.Digest != digest {
			return errors.Errorf("Digest (%s) specified for %s does not match remote image's (%s)", t.Digest, t.Source, digest)
		}
		if t.Size != size {
			return errors.Errorf("Size (%d) specified for %s does not match remote image's (%s)", t.Size, t.Source, size)
		}

		dest := "docker://" + repo + "/mos:" + dropHashAlg(digest)
		copyOpts := lib.ImageCopyOpts{
			Src: t.Source,
			Dest:        dest,
			Progress:    os.Stdout,
			SrcSkipTLS:  true,
			DestSkipTLS: true,
		}
		if err := lib.ImageCopy(copyOpts); err != nil {
			return errors.Wrapf(err, "Failed copying %s to %s", t.Source, dest)
		}
		install.Targets = append(install.Targets, Target{
			ServiceName: t.ServiceName,
			Version:     t.Version,
			ServiceType: t.ServiceType,
			Network:     t.Network,
			NSGroup:     t.NSGroup,
			Digest:      digest,
			Size:        size},
		)
	}

	workdir, err := os.MkdirTemp("", "manifest")
	if err != nil {
		return errors.Wrapf(err, "Failed creating tempdir")
	}
	defer os.RemoveAll(workdir)

	filePath := filepath.Join(workdir, "install.json")
	f, err := os.OpenFile(filePath, os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "Failed opening %s for writing", filePath)
	}
	err = json.NewEncoder(f).Encode(install)
	if err != nil {
		f.Close()
		return errors.Wrapf(err, "Failed encoding the install.json")
	}
	f.Close()
	signPath := filepath.Join(workdir, "install.json.signed")
	if err = trust.Sign(filePath, signPath, key); err != nil {
		return errors.Wrapf(err, "Failed signing file")
	}

	dest := repo + "/" + destpath
	if err = PostManifest(workdir, cert, dest); err != nil {
		return errors.Wrapf(err, "Failed writing install.json to %s", dest)
	}

	return nil
}

func getSizeDigestOCI(inUrl string) (string, int64, error) {
	split := strings.SplitN(inUrl, ":", 3)
	if len(split) != 3 {
		return "", 0, errors.Errorf("Bad oci url: %s", inUrl)
	}
	ocidir := split[1]
	image := split[2]
	oci, err := umoci.OpenLayout(ocidir)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Failed opening oci layout at %q", ocidir)
	}
	dp, err := oci.ResolveReference(context.Background(), image)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Failed looking up image %q", image)
	}
	if len(dp) != 1 {
		return "", 0, errors.Errorf("bad descriptor tag %q", image)
	}
	blob, err := oci.FromDescriptor(context.Background(), dp[0].Descriptor())
	if err != nil {
		return "", 0, errors.Wrapf(err, "Error finding image")
	}
	defer blob.Close()
	desc := blob.Descriptor
	return desc.Digest.String(), desc.Size, nil
}

func getSizeDigestDist(inUrl string) (string, int64, error) {
	// http://127.0.0.1:18080/v2/os/busybox-squashfs/manifests/1.0
	r, err := NewOciRepo(inUrl)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Failed to find source repo info for %q", inUrl)
	}

	u, err := r.pickUrl(inUrl)
	if err != nil {
		return "", 0, errors.Wrapf(err, "Error parsing install manifest inUrl %q", inUrl)
	}

	return u.mDigest, u.mSize, nil
}

func getSizeDigest(inUrl string) (string, int64, error) {
	if strings.HasPrefix(inUrl, "oci:") {
		return getSizeDigestOCI(inUrl)
	}
	return getSizeDigestDist(inUrl)
}

func PostManifest(workdir, certPath, dest string) error {
	// Post the manifest
	// Post the certificate
	// Post the signature
	return nil
}

func dropHashAlg(d string) string {
	s := strings.SplitN(d, ":", 2)
	if len(s) == 2 {
		return s[1]
	}
	return d
}
