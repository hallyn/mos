package main

import (
	"fmt"

	"github.com/project-machine/mos/pkg/mosconfig"
	"github.com/urfave/cli"
)

// How this is meant to be used:
// 1. write a targets.yaml:
// version: 1
// targets:
//  - source: docker://ubuntu:latest
//    imagepath: puzzleos/hostfs
//    version: 1.0.0
//  - source: oci:/oci:ran
//    imagepath: puzzleos/ran
//    version: 0.9
//    manifest_hash: sha256:xxx  # optional
//  - source: docker://busybox:latest
//    imagepath: extra/guestfs
//    version:1.0
//
// 2. collect those targets into an oci repo.
// # mosb manifest collect-targets --file=targets.yaml --repo=10.3.1.25:5000
//
// This will copy each of the listed 'source' images to the OCI
// distribution compliant registry at 10.3.1.25:5000 under the
// name $imagepath with tag $version.
//
// 3. write an install manifest.yaml:
// version: 1
// product: de6c82c5-2e01-4c92-949b-a6545d30fc06
// update_type: complete
// targets:
//   - service_name: hostfs
//     imagepath: puzzleos/hostfs
//     version: 1.0.0
//     manifest_hash: xxx  # not optional here
//     service_type: hostfs
//     nsgroup: none
//     network:
//       type: none
//     mounts: []
//   - service_name: hostfs
//     imagepath: puzzleos/hostfs
//     version: 1.0.0
//     manifest_hash: xxx
//     service_type: hostfs
//     nsgroup: ran
//     network:
//       type: none
//     mounts:
//       source: /var/ran
//       dest: /content/ran
//
// 4. sign and publish the install manifest:
// # mosb manifest publish --file=manifest.yaml --cert=cert.pem --key=privkey.pem \
//   --repo=10.3.1.25:5000 --path=machine/install:1.0.0
//
// This will sign the manifest.yaml, publish it to 10.3.1.25:500
// as an artifact (manifest) called machine/install:1.0.0.  In
// addition, there will be two artifacts referring to this manifest,
// one containing cert.pem and the other containing the signature
// of the manifest.yaml with privkey.pem.
//
// Note, this is also where we would convert manifest.yaml into
// an install.json.

var manifestCmd = cli.Command{
	Name:  "manifest",
	Usage: "build and publish a mos install manifest",
	Subcommands: []cli.Command{
		cli.Command{
			Name:   "collect-targets",
			Usage: "Collect targets for an install",
			Action: doCollectLayers,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file",
					Usage: "filename for targets.yaml to use",
					Value: "targets.yaml",
				},
				cli.StringFlag{
					Name:  "repo",
					Usage: "address:port for the OCI repository to write to, e.g. 10.0.2.2:5000",
				},
			},
		},
		cli.Command{
			Name:   "publish",
			Action: doPublishManifest,
			Usage: "build and publish an install manifest",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file",
					Usage: "filename for install.yaml to publish",
					Value: "install.yaml",
				},
				cli.StringFlag{
					Name:  "key",
					Usage: "path to manifest signing key to use",
					Value: "",
				},
				cli.StringFlag{
					Name:  "cert",
					Usage: "path to manifest certificate to use",
					Value: "",
				},
				cli.StringFlag{
					Name:  "repo",
					Usage: "address:port for the OCI repository to write to, e.g. 10.0.2.2:5000",
				},
				cli.StringFlag{
					Name:  "dest-path",
					Usage: "path on OCI repository to write the manifest to, e.g. puzzleos/hostfs:1.0.1",
				},
			},
		},
	},
}

func doCollectLayers(ctx *cli.Context) error {
	return fmt.Errorf("Not yet implemented")
}

func doPublishManifest(ctx *cli.Context) error {
	return mosconfig.PublishManifest(ctx)
}
