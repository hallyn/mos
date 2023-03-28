package main

import (
	"github.com/project-machine/mos/pkg/mosconfig"
	"github.com/urfave/cli"
)

var installCmd = cli.Command{
	Name:   "install",
	Usage:  "install a new mos system",
	Action: doInstall,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config-dir, c",
			Usage: "Directory where mos config is found",
			Value: "/config",
		},
		cli.StringFlag{
			Name:  "atomfs-store, a",
			Usage: "Directory under which atomfs store is kept",
			Value: "/atomfs-store",
		},
		cli.StringFlag{
			Name:  "ca-path",
			Usage: "Path to a manifest CA file.",
			Value: "/factory/secure/manifestCa.pem",
		},
	},
}

func doInstall(ctx *cli.Context) error {
	if err := mosconfig.InitializeMos(ctx); err != nil {
		return err
	}

	return nil
}
