package service

import (
	"errors"
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/drep-project/DREP-Chain/app"
	blockmgr "github.com/drep-project/DREP-Chain/blockmgr"
	accountService "github.com/drep-project/DREP-Chain/pkgs/accounts/service"
	"github.com/drep-project/DREP-Chain/pkgs/drepclient/component/console"
	cliTypes "github.com/drep-project/DREP-Chain/pkgs/drepclient/types"
	"github.com/drep-project/DREP-Chain/pkgs/log"
	rpc2 "github.com/drep-project/DREP-Chain/pkgs/rpc"
	"github.com/drep-project/rpc"
	"gopkg.in/urfave/cli.v1"
)

var (
	ConfigFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TODO add config description",
	}
)

// CliService provides an interactive command line window
type CliService struct {
	Config         *cliTypes.Config
	Log            *log.LogService                `service:"log"`
	Blockmgr       *blockmgr.BlockMgr             `service:"blockmgr"`
	AccountService *accountService.AccountService `service:"accounts"`
	RpcService     *rpc2.RpcService               `service:"rpc"`

	console *console.Console
}

// Name name
func (cliService *CliService) Name() string {
	return "cli"
}

// Api api none
func (cliService *CliService) Api() []app.API {
	return []app.API{}
}

// Flags flags  enable load js and execute before run
func (cliService *CliService) CommandFlags() ([]cli.Command, []cli.Flag) {
	defaultFlags := []cli.Flag{cliTypes.JSpathFlag, cliTypes.ExecFlag, cliTypes.PreloadJSFlag}
	consoleCommand := cli.Command{
		Name:     "console",
		Usage:    "Start an interactive JavaScript environment",
		Flags:    []cli.Flag{},
		Category: "CONSOLE COMMANDS",
		Description: `
The Drep console is an interactive shell for the JavaScript runtime environment
which exposes a node admin interface as well as the Ðapp JavaScript API.
See https://github.com/ethereum/go-ethereum/wiki/JavaScript-Console.`,
	}

	attachCommand := cli.Command{
		Name:      "attach",
		Usage:     "Start an interactive JavaScript environment (connect to node)",
		ArgsUsage: "[endpoint]",
		Flags:     []cli.Flag{},
		Category:  "CONSOLE COMMANDS",
		Description: `
The Drep console is an interactive shell for the JavaScript runtime environment
which exposes a node admin interface as well as the Ðapp JavaScript API.
See https://github.com/ethereum/go-ethereum/wiki/JavaScript-Console.
This command allows to open a console on a running drep node.`,
	}
	return []cli.Command{consoleCommand, attachCommand}, defaultFlags
}

func (cliService *CliService) P2pMessages() map[int]interface{} {
	return map[int]interface{}{}
}

// Init  set console config
func (cliService *CliService) Init(executeContext *app.ExecuteContext) error {
	return nil
}

func (cliService *CliService) Start(executeContext *app.ExecuteContext) error {
	if executeContext.Cli.Command.Name == "console" {
		return cliService.localConsole(executeContext)
	} else if executeContext.Cli.Command.Name == "attach" {
		return cliService.remoteConsole(executeContext)
	} else {
		return cliService.drep(executeContext)
	}
}

func (cliService *CliService) Stop(executeContext *app.ExecuteContext) error {
	if cliService.console != nil {
		cliService.console.Stop(false)
	}
	console.Stdin.Close()
	return nil
}

func (cliService *CliService) Receive(context actor.Context) {}

func (cliService *CliService) localConsole(executeContext *app.ExecuteContext) error {
	if !cliService.RpcService.Config.IPCEnabled {
		return errors.New("ipc must be enable in console mode")
	}
	// Attach to the newly started node and start the JavaScript console
	if cliService.RpcService.IpcHandler == nil {
		return errors.New("Failed to attach to the inproc drep")
	}
	client := rpc.DialInProc(cliService.RpcService.IpcHandler)
	config := console.Config{
		HomeDir: executeContext.CommonConfig.HomeDir,
		DocRoot: executeContext.Cli.GlobalString(cliTypes.JSpathFlag.Name),
		Client:  client,
		Preload: cliTypes.MakeConsolePreloads(executeContext.Cli),
	}

	var err error
	cliService.console, err = console.New(config)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to start the JavaScript console: %v", err))
	}
	// If only a short execution was requested, evaluate and return
	if script := executeContext.Cli.GlobalString(cliTypes.ExecFlag.Name); script != "" {
		cliService.console.Evaluate(script)
		return nil
	}
	// Otherwise print the welcome screen and enter interactive mode
	cliService.console.Welcome()
	cliService.console.Interactive(executeContext.Quit)
	return nil
}

// remoteConsole will connect to a remote drep instance, attaching a JavaScript
// console to it.
func (cliService *CliService) remoteConsole(executeContext *app.ExecuteContext) error {
	endpoint := executeContext.Cli.Args().First()
	if len(endpoint) == 0 {
		return fmt.Errorf("You have to specify an address")
	}
	client, err := rpc.Dial(endpoint)
	if err != nil {
		return fmt.Errorf("Unable to attach to remote drep: %v", err)
	}

	path := executeContext.CommonConfig.HomeDir
	cliService.Config = &cliTypes.Config{}
	cliService.Config.Config = console.Config{
		HomeDir: path,
		DocRoot: executeContext.Cli.GlobalString(cliTypes.JSpathFlag.Name),
		Client:  client,
		Preload: cliTypes.MakeConsolePreloads(executeContext.Cli),
	}

	cliService.console, err = console.New(cliService.Config.Config)
	if err != nil {
		return fmt.Errorf("Failed to start the JavaScript console: %v", err)
	}
	defer cliService.console.Stop(false)

	if script := executeContext.Cli.GlobalString(cliTypes.ExecFlag.Name); script != "" {
		cliService.console.Evaluate(script)
		return nil
	}

	// Otherwise print the welcome screen and enter interactive mode
	cliService.console.Welcome()
	cliService.console.Interactive(executeContext.Quit)

	return nil
}

// drep is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func (cliService *CliService) drep(executeContext *app.ExecuteContext) error {
	return nil
}
