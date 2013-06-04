package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/ty/fun"
)

func install() {
	flag.Usage = usage("Usage: wingo-contrib install script-name")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	scriptName := flag.Arg(0)

	cpath := contribPath()
	gt, lt := getGithubTree(), getLocalTree(cpath)
	if !gt.scriptExists(scriptName) {
		log.Fatalf("Script '%s' does not exist in wingo-contrib.", scriptName)
	}
	if lt.scriptExists(scriptName) {
		log.Fatalf("Script '%s' is already installed. Please use "+
			"the upgrade command to update.", scriptName)
	}

	// Make the directory, download the files, copy them over, make executable.
	scriptDir := filepath.Join(cpath, scriptName)
	if err := os.Mkdir(scriptDir, 0777); err != nil {
		log.Fatalf("Could not create '%s': %s", scriptDir, err)
	}

	script := gt.download(scriptName)
	script.copyAll(scriptDir)
	chmodx(filepath.Join(scriptDir, scriptName))
}

func upgrade() {
	var flagSkipConfig = false
	flag.BoolVar(&flagSkipConfig, "skip-config", flagSkipConfig,
		"Upgrade proceeds even if the local and remote config files differ.")

	flag.Usage = usage("Usage: wingo-contrib upgrade script-name [flags]")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	scriptName := flag.Arg(0)

	cpath := contribPath()
	gt, lt := getGithubTree(), getLocalTree(cpath)
	if !gt.scriptExists(scriptName) {
		log.Fatalf("Script '%s' does not exist in wingo-contrib.", scriptName)
	}
	if !lt.scriptExists(scriptName) {
		log.Fatalf("Script '%s' is not installed. Please add it "+
			"with the install command first.", scriptName)
	}

	script := gt.download(scriptName)
	scriptDir := filepath.Join(cpath, scriptName)
	lnode := localNode{scriptName, scriptDir}

	// Don't touch the local configuration file when set.
	if flagSkipConfig {
		for fname, _ := range script.Files {
			if fname != configName(scriptName) {
				script.copyFile(fname, filepath.Join(scriptDir, fname))
			}
		}
		return
	}

	// If the local script has no configuration, then we can overwrite all of
	// the files willy-nilly. Similarly if the remote script has no
	// configuration.
	// We can also do this when the configuration file hasn't been changed.
	if !lnode.hasConfig() ||
		script.config() == nil ||
		bytes.Equal(lnode.readConfig(), script.config()) {

		script.copyAll(scriptDir)
		return
	}

	// At this point, there are config files both locally and in the remote
	// repo *and* they are different. So we've got to throw our hands up
	// and require manual intervention.
	log.Println("MANUAL INTERVENTION REQUIRED!")
	log.Printf("The configuration file\n\n    %s\n\ndiffers from the remote "+
		"copy.\n", filepath.Join(lnode.Path, configName(lnode.ScriptName)))
	log.Printf("Please move the local configuration to a different location\n" +
		"and run the upgrade command again. Then merge your old\n" +
		"configuration file with the new one.\n")
	log.Println("Alternatively, upgrade with '--skip-config' set.\n")
	log.Println("(Holler at BurntSushi if you think this behavior " +
		"should change.)")
}

func search() {
	flag.Usage = usage("Usage: wingo-contrib search [query]\n\n" +
		"An empty query shows all available scripts.")
	flag.Parse()
	query := strings.Join(flag.Args(), " ")

	type script struct {
		name string
		desc string
	}
	dled := func(sname string) script {
		return script{
			name: sname,
			desc: readDescription(downloadFile(sname, "README.md")),
		}
	}
	less := func(s1, s2 script) bool { return s1.name < s2.name }

	scripts := fun.ParMap(dled, getGithubTree().scripts()).([]script)
	scripts = fun.QuickSort(less, scripts).([]script)

	for _, script := range scripts {
		if strings.Contains(strings.ToLower(script.desc), query) {
			fmt.Printf("%s\n    %s\n\n",
				script.name, strings.Replace(script.desc, "\n", "\n    ", -1))
		}
	}
}

func info() {
	flag.Usage = usage("Usage: wingo-contrib info script-name")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	scriptName := flag.Arg(0)

	gt := getGithubTree()
	if !gt.scriptExists(scriptName) {
		log.Fatalf("Script '%s' does not exist in wingo-contrib.", scriptName)
	}

	fmt.Printf("\n%s\n", downloadFile(scriptName, "README.md"))
}

func usage(info string) func() {
	return func() {
		log.Printf("%s\n\n", info)
		flag.VisitAll(func(fl *flag.Flag) {
			var def string
			if len(fl.DefValue) > 0 {
				def = fmt.Sprintf(" (default: %s)", fl.DefValue)
			}

			usage := strings.Replace(fl.Usage, "\n", "\n    ", -1)
			log.Printf("-%s%s\n", fl.Name, def)
			log.Printf("    %s\n", usage)
		})
		os.Exit(1)
	}
}

func list() {
	flag.Usage = usage("Usage: wingo-contrib list")
	flag.Parse()

	cpath := contribPath()
	gt, lt := getGithubTree(), getLocalTree(cpath)
	for _, lnode := range lt {
		if gt.scriptExists(lnode.ScriptName) {
			fmt.Println(lnode.ScriptName)
		}
	}
}

// chmodx sets the executable bit for owner, group and other.
func chmodx(fpath string) {
	fi, err := os.Stat(fpath)
	if err != nil {
		log.Fatal(err)
	}

	perms := fi.Mode().Perm()
	for i := 6; i >= 0; i -= 3 {
		if perms&(1<<uint(i)) == 0 {
			perms |= 1 << uint(i)
		}
	}
	if err := os.Chmod(fpath, perms); err != nil {
		log.Fatalf("Could not make '%s' executable: %s", fpath, err)
	}
}
