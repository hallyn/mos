package main

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/project-machine/mos/pkg/utils"
	"github.com/urfave/cli"
)

var sudiCmd = cli.Command{
	Name:      "sudi",
	Usage:     "Generate and sign sudi cert",
	UsageText: "CACert, private-key, output-dir, product-uuid [, machine-uuid]",
	Subcommands: []cli.Command{
		cli.Command{
			Name:      "list",
			Action:    doListSudi,
			Usage:     "list sudi keys",
			ArgsUsage: "<keyset-name> <project-name>",
		},
		cli.Command{
			Name:      "add",
			Action:    doGenSudi,
			Usage:     "add a new sudi key to project",
			ArgsUsage: "<keyset-name> <project-name> [<serial-number>|uuid]",
		},
	},
}

// ~/.local/share/machine/trust/keys/
//
//	keyset1/manifest/project-name/{uuid,privkey.pem,cert.pem}
//	keyset1/manifest/project-name/sudi/host-serial/{uuid,privkey.pem,cert.pem}
func doGenSudi(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 && len(args) != 3 {
		return fmt.Errorf("Wrong number of arguments (see \"--help\")")
	}
	keysetName := args[0]
	projName := args[1]
	var myUUID string
	if len(args) == 3 {
		myUUID = args[2]
	}

	trustDir, err := utils.GetMosKeyPath()
	if err != nil {
		return err
	}
	keysetPath := filepath.Join(trustDir, keysetName)
	if !utils.PathExists(keysetPath) {
		return fmt.Errorf("Keyset not found: %s", keysetName)
	}

	projPath := filepath.Join(keysetPath, "manifest", projName)
	if !utils.PathExists(projPath) {
		return fmt.Errorf("Project not found: %s", projName)
	}

	if _, err = genSudi(keysetPath, projPath, myUUID); err != nil {
		return errors.Wrapf(err, "Failed generating SUDI")
	}

	return nil
}

// Generate a SUDI key for given uuid.  If uuid is "", then generate a
// new UUID.  Return the directory path for this new SUDI cert.
func genSudi(keysetPath, projDir, sudiUUID string) (string, error) {
	prodUUID, err := os.ReadFile(filepath.Join(projDir, "uuid"))
	if err != nil {
		return "", errors.Wrapf(err, "Failed reading project UUID")
	}

	if sudiUUID == "" {
		sudiUUID = uuid.NewString()
	}

	// read the project CA certificate
	capath := filepath.Join(keysetPath, "sudi-ca")
	caCert, err := readCertificateFromFile(filepath.Join(capath, "cert.pem"))
	if err != nil {
		return "", errors.Wrapf(err, "Failed reading SUDI CA certificate")
	}

	// read the project CA private key to sign the sudi key with
	caKey, err := readPrivKeyFromFile(filepath.Join(capath, "privkey.pem"))
	if err != nil {
		return "", errors.Wrapf(err, "Failed reading SUDI CA key")
	}

	certTmpl := newCertTemplate(string(prodUUID), sudiUUID)

	snPath := filepath.Join(projDir, "sudi", sudiUUID)
	if err := utils.EnsureDir(snPath); err != nil {
		return "", errors.Wrapf(err, "Failed creating new SUDI directory")
	}

	if err := SignCert(&certTmpl, caCert, caKey, snPath); err != nil {
		os.RemoveAll(snPath)
		return "", errors.Wrapf(err, "Failed creating new SUDI keypair")
	}

	return snPath, nil
}

func newCertTemplate(productUUID, machineUUID string) x509.Certificate {
	return x509.Certificate{
		Subject: pkix.Name{
			SerialNumber: fmt.Sprintf("PID:%s SN:%s", productUUID, machineUUID),
			CommonName:   machineUUID,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Date(2099, time.December, 31, 23, 0, 0, 0, time.UTC),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
}

func doListSudi(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return fmt.Errorf("Wrong number of arguments (see \"--help\")")
	}
	keysetName := args[0]
	projName := args[1]
	trustDir, err := utils.GetMosKeyPath()
	if err != nil {
		return err
	}
	keysetPath := filepath.Join(trustDir, keysetName)
	if !utils.PathExists(keysetPath) {
		return fmt.Errorf("Keyset not found: %s", keysetName)
	}

	projPath := filepath.Join(keysetPath, "manifest", projName)
	if !utils.PathExists(projPath) {
		return fmt.Errorf("Project not found: %s", projName)
	}

	dir := filepath.Join(projPath, "sudi")
	serials, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("Failed reading sudi directory %q: %w", dir, err)
	}

	for _, sn := range serials {
		fmt.Printf("%s\n", sn.Name())
	}

	return nil
}
