package main

import (
	"flag"
	"fmt"
	"github.com/DharmaOfCode/gorp/base"
	"github.com/DharmaOfCode/gorp/debugger"
	"github.com/DharmaOfCode/gorp/modules"
	"github.com/spf13/viper"
	"github.com/wirepair/gcd"
	"github.com/wirepair/gcd/gcdapi"
	"log"
	"os"
	"strings"
)

type State struct {
	Debugger debugger.Debugger
	Modules  modules.Modules
	ModPath  string
	Run      bool
	GetInfo  bool
}

var (
	cfgFile string
	config  *base.Configuration

	testPath string
	testDir  string
	testPort string
)

func init() {
	flag.StringVar(&testPath, "chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "path to chrome")
	flag.StringVar(&testDir, "dir", "/tmp/chrome-testing", "user directory")
	flag.StringVar(&testPort, "port", "9222", "Debugger port")
}

func ParseCmdLine() *State {
	s := State{}
	flag.StringVar(&cfgFile, "c", "", "configuration file path")
	flag.StringVar(&s.ModPath, "m", "", "path of module to get info for")
	flag.BoolVar(&s.Run, "r", true, "run gorp")
	flag.BoolVar(&s.GetInfo, "i", false, "run gorp")

	flag.Parse()
	return &s
}

func RunGorp(s *State) {
	initConfig()
	var err error

	// Load the modules
	s.Modules = modules.Modules{}
	err = s.Modules.InitProcessors(config.Modules.Processors)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = s.Modules.InitInspectors(config.Modules.Inspectors)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Setup the debugger
	s.Debugger = debugger.Debugger{
		Modules: s.Modules,
	}
	s.Debugger.Options = debugger.Options{
		Verbose:       config.Verbose,
		EnableConsole: true,
	}

	// TODO: This should be abstracted in the debugger struct
	s.Debugger.ChromeProxy = startGcd()
	defer s.Debugger.ChromeProxy.ExitProcess()

	s.Debugger.StartTarget()
	//Create a channel to be able to signal a termination to our Chrome connection
	s.Debugger.Done = make(chan bool)
	shouldWait := true

	patterns := make([]*gcdapi.NetworkRequestPattern, 2)
	//Default is everything!
	docPattern := "*"
	jsPattern := "*"
	if config.Scope != "" {
		docPattern = "*" + config.Scope + "/*"
		jsPattern = "*" + config.Scope + "*.js"
	}
	patterns[0] = &gcdapi.NetworkRequestPattern{
		UrlPattern:        docPattern,
		ResourceType:      "Document",
		InterceptionStage: "HeadersReceived",
	}
	patterns[1] = &gcdapi.NetworkRequestPattern{
		UrlPattern:        jsPattern,
		ResourceType:      "Script",
		InterceptionStage: "HeadersReceived",
	}
	interceptParams := &gcdapi.NetworkSetRequestInterceptionParams{Patterns: patterns}

	s.Debugger.SetupRequestInterception(interceptParams)

	if shouldWait {
		log.Println("[+] Wait for events...")
		<-s.Debugger.Done
	}
}

func GetModInfo(s *State) {
	s.Modules = modules.Modules{}
	if strings.Contains(s.ModPath, "processors") {
		p, err := s.Modules.GetProcessor(s.ModPath)
		if err != nil {
			log.Println("[+] Unable to find processor " + s.ModPath)
		} else {
			p.ShowInfo()
		}
	} else if strings.Contains(s.ModPath, "inspectors") {
		i, err := s.Modules.GetInspector(s.ModPath)
		if err != nil {
			log.Println("[+] Unable to find processor " + s.ModPath)
		} else {
			i.ShowInfo()
		}
	} else {
		log.Println("[+] Unable to find module " + s.ModPath)
	}

	fmt.Println(s.ModPath)
}

func main() {
	s := ParseCmdLine()
	if s.GetInfo {
		GetModInfo(s)
	} else {
		RunGorp(s)
	}
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find in base
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
	}

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
		os.Exit(1)
	}

	err = viper.Unmarshal(&config)
}

// TODO: Move this to debugger
func startGcd() *gcd.Gcd {
	testDir = "/tmp/chrome-testing"
	testPort = "9222"
	debugger := gcd.NewChromeDebugger()
	//debugger.DeleteProfileOnExit()
	debugger.AddFlags(config.Flags)
	debugger.StartProcess(testPath, testDir, testPort)
	return debugger
}
