package main

import (
	"fmt"

	"github.com/project-machine/mos/pkg/mosconfig"
	"github.com/urfave/cli"
)

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
