package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/h44z/wg-portal/internal/portal"
	"github.com/pkg/errors"

	"github.com/urfave/cli/v2"
)

const (
	dsnFlag       = "dsn"
	interfaceFlag = "interface"
)

var backend *portal.Backend

var globalFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  dsnFlag,
		Value: "./sqlite.db",
		Usage: "A DSN for the data store.",
	},
}

var commands = []*cli.Command{
	{
		Name:    "list",
		Aliases: []string{"l"},
		Usage:   "list interfaces or peers",
		Subcommands: []*cli.Command{
			{
				Name:      "interface",
				Usage:     "show interface information",
				ArgsUsage: "<interface identifier>",
				Action: func(c *cli.Context) error {
					if c.Args().Len() != 1 {
						return errors.New("missing/invalid interface identifier")
					}
					interfaceIdentifier := persistence.InterfaceIdentifier(strings.TrimSpace(c.Args().Get(0)))

					cfg, err := backend.GetInterface(interfaceIdentifier)
					if err != nil {
						return errors.WithMessage(err, "failed to get interface")
					}

					peers, err := backend.GetPeers(interfaceIdentifier)
					if err != nil {
						return errors.WithMessage(err, "failed to get interface peers")
					}

					config, err := backend.GetInterfaceConfig(cfg, peers)
					if err != nil {
						return errors.WithMessage(err, "failed to get interface config")
					}

					fmt.Println(config)

					return nil
				},
			},
			{
				Name:  "interfaces",
				Usage: "list all interfaces",
				Action: func(c *cli.Context) error {
					interfaces, err := backend.GetInterfaces()
					if err != nil {
						return errors.WithMessage(err, "failed to get all interfaces")
					}

					fmt.Println("Managed WireGuard Interfaces:")
					for i, cfg := range interfaces {
						desc := ""
						if cfg.DisplayName != "" {
							desc = fmt.Sprintf(" (%s)", cfg.DisplayName)
						}
						fmt.Printf(" %d\t%s%s\n", i, cfg.Identifier, desc)
					}

					importable, err := backend.GetImportableInterfaces()
					if err != nil {
						return errors.WithMessage(err, "failed to get importable interfaces")
					}

					fmt.Println("Importable WireGuard Interfaces:")
					i := 0
					for cfg := range importable {
						fmt.Printf(" %d\t%s\n", i, cfg.Identifier)
						i++
					}

					return nil
				},
			},
			{
				Name:      "peers",
				Usage:     "list all peers",
				ArgsUsage: "<interface identifier>",
				Action: func(c *cli.Context) error {
					if c.Args().Len() != 1 {
						return errors.New("missing/invalid interface identifier")
					}
					interfaceIdentifier := persistence.InterfaceIdentifier(strings.TrimSpace(c.Args().Get(0)))

					peers, err := backend.GetPeers(interfaceIdentifier)
					if err != nil {
						return errors.WithMessage(err, "failed to get all peers")
					}

					fmt.Println("WireGuard Peers:")
					for i, cfg := range peers {
						desc := ""
						if cfg.DisplayName != "" {
							desc = fmt.Sprintf(" (%s)", cfg.DisplayName)
						}
						fmt.Printf(" %d\t%s%s\n", i, cfg.Identifier, desc)
					}
					return nil
				},
			},
		},
	},
	{
		Name:      "import",
		Aliases:   []string{"i"},
		Usage:     "import existing interface",
		ArgsUsage: "<interface identifier>",
		Action: func(c *cli.Context) error {
			if c.Args().Len() != 1 {
				return errors.New("missing/invalid interface identifier")
			}
			importIdentifier := strings.TrimSpace(c.Args().Get(0))

			err := backend.ImportInterface(persistence.InterfaceIdentifier(importIdentifier))
			if err != nil {
				return err
			}

			fmt.Println("Imported interface", importIdentifier)

			return nil
		},
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "wg-portal"
	app.Version = "0.0.1"
	app.Usage = "WireGuard Portal CLI client"
	app.EnableBashCompletion = true
	app.Commands = commands
	app.Flags = globalFlags
	app.Before = func(c *cli.Context) error {
		dsn := c.String(dsnFlag)
		database, err := persistence.NewDatabase(persistence.DatabaseConfig{
			Type: "sqlite",
			DSN:  dsn,
		})
		if err != nil {
			return errors.WithMessagef(err, "failed to initialize persistent store")
		}

		backend, err = portal.NewBackend(database)
		if err != nil {
			return errors.WithMessagef(err, "backend failed to initialize")
		}
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
