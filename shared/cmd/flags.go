package cmd

import (
	"github.com/ethereum/go-ethereum/node"
	"github.com/urfave/cli"
)

var (
	// IPCPathFlag defines the filename of a pipe within the datadir.
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
	}
	// DataDirFlag defines a path on disk.
	DataDirFlag = DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DirectoryString{node.DefaultDataDir()},
	}
	// NetworkIdFlag defines the specific network identifier.
	NetworkIdFlag = cli.Uint64Flag{
		Name:  "networkid",
		Usage: "Network identifier (integer, 1=Frontier, 2=Morden (disused), 3=Ropsten, 4=Rinkeby)",
		Value: 1,
	}
	// PasswordFileFlag defines the path to the user's account password file.
	PasswordFileFlag = cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-interactive password input",
		Value: "",
	}
)
