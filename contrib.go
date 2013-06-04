package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/ty/fun"
	"github.com/BurntSushi/xdg"
)

const (
	treeUrl    = "https://api.github.com/repos/wingowm/contrib"
	treeParams = "/git/trees/master?recursive=1"
	rawPrefix  = "https://raw.github.com/wingowm/contrib/master"
)

// scriptPaths locates the script configuration directory.
// Note that the fallback path is not the import path of *this*
// package, but rather the Wingo package.
var scriptPaths = xdg.Paths{
	Override:     "",
	XDGSuffix:    "wingo",
	GoImportPath: "github.com/BurntSushi/wingo/config",
}

// githubTree represents the entire wingo-contrib repository.
type githubTree struct {
	SHA  string
	Url  string
	Tree []githubNode
}

// githubNode represents an entry in the remote repository. It is not
// necessarily a script.
type githubNode struct {
	Mode string
	Type string
	SHA  string
	Path string
	Size int
	Url  string
}

// getGithubTree recursively fetches all entries in the wingo-contrib
// repository.
func getGithubTree() *githubTree {
	resp, err := http.Get(treeUrl + treeParams)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var tree *githubTree
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&tree); err != nil {
		log.Fatal(err)
	}
	return tree
}

// scriptExists returns true if a script with the given name exists in
// the remote repository.
func (gt *githubTree) scriptExists(scriptName string) bool {
	for _, node := range gt.Tree {
		sname, _ := node.split()
		if node.isScript() && sname == scriptName {
			return true
		}
	}
	return false
}

// scripts returns all scripts for the given repository.
func (gt *githubTree) scripts() []string {
	snames := make(map[string]bool)
	for _, node := range gt.Tree {
		if node.isScript() {
			sname, _ := node.split()
			snames[sname] = true
		}
	}
	ss := fun.Keys(snames).([]string)
	sort.Sort(sort.StringSlice(ss))
	return ss
}

// isScript returns true if the given entry is a script.
func (node githubNode) isScript() bool {
	return strings.Count(node.Path, "/") == 1
}

// split returns the script name and base file name of the corresponding
// entry in the GitHub repo.
func (node githubNode) split() (scriptName, fileName string) {
	scriptName, fileName = filepath.Split(node.Path)
	scriptName = filepath.Clean(scriptName)
	return scriptName, fileName
}

// localTree represents the set of all scripts installed in the local
// file system.
type localTree []localNode

// localNode represents a script installed in the local file system.
type localNode struct {
	ScriptName string
	Path       string
}

// scriptExists returns true if the script provided exists in the local
// file system. Returns false otherwise.
func (lt localTree) scriptExists(scriptName string) bool {
	for _, node := range lt {
		if node.ScriptName == scriptName {
			return true
		}
	}
	return false
}

// getLocalTree scans the local file system for installed scripts.
func getLocalTree(cpath string) localTree {
	localDir, err := os.Open(cpath)
	if err != nil {
		log.Fatal(err)
	}

	fis, err := localDir.Readdir(0)
	if err != nil {
		log.Fatal(err)
	}

	var nodes []localNode
	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}
		p := filepath.Join(cpath, fi.Name())
		nodes = append(nodes, localNode{fi.Name(), p})
	}
	return localTree(nodes)
}

// hasConfig returns true if a configuration file for this script exists
// in the local file system.
func (ln localNode) hasConfig() bool {
	_, err := os.Stat(filepath.Join(ln.Path, configName(ln.ScriptName)))
	return err == nil
}

// readConfig returns the contents of the configuration file.
// If there is a problem reading the config file (e.g., if it doesn't exist)
// then the program stops with an error message.
func (ln localNode) readConfig() []byte {
	fp := filepath.Join(ln.Path, configName(ln.ScriptName))
	contents, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatalf("Could not read config file %s: %s", fp, err)
	}
	return contents
}

// contribPath returns the absolute path to the `scripts` directory.
// If the directory does not exist, the program stops.
func contribPath() string {
	fp, err := scriptPaths.ConfigFile("scripts")
	if err != nil {
		log.Fatalf("Could not find local scripts directory: %s", err)
	}
	return fp
}

// contribScript is an in-memory representation of an entire script.
// This includes the script program itself, the readme and the config
// plus any other files that might be in the script directory.
// A contribScript *must* have non-nil README.md and `scriptName` entries.
type contribScript struct {
	Name  string
	Files map[string][]byte
}

func (s *contribScript) source() []byte {
	return s.Files[s.Name]
}

func (s *contribScript) config() []byte {
	return s.Files[configName(s.Name)]
}

// readDescription parses a README.md file and extracts only the description.
func readDescription(readme []byte) string {
	noDesc := bytes.Split(readme, []byte("Description\n===========\n"))
	noEx := bytes.Split(noDesc[1], []byte("\n\n\n"))
	return string(bytes.TrimSpace(noEx[0]))
}

// copyFile copies the file in memory to a new file at dest.
// The file name should be a key in the contribScript.Files map.
// If a file already exists at dest, it is overwritten.
// If an error occurs, the program stops with an error message.
func (s *contribScript) copyFile(fileName, dest string) {
	contents := s.Files[fileName]
	if contents == nil {
		log.Fatal("BUG: Could not find file %s in memory.", fileName)
	}

	destf, err := os.Create(dest)
	if err != nil {
		log.Fatal("Could not create file %s: %s", dest, err)
	}
	defer destf.Close()

	if _, err := io.Copy(destf, bytes.NewReader(contents)); err != nil {
		log.Fatal("Could not write to file %s: %s", dest, err)
	}
}

// copyAll runs copyFile on every file in the script directory.
func (s *contribScript) copyAll(scriptDir string) {
	for fname, _ := range s.Files {
		s.copyFile(fname, filepath.Join(scriptDir, fname))
	}
}

// download retrieves all of the files in a script directory.
// If there are any problems downloading the files, then the program stops
// with an error message.
// If either the `README.md` or `{scriptFile}` files are not present, then
// the program stops with an error message. (This is a corrupt repository.)
func (gt *githubTree) download(scriptName string) *contribScript {
	script := &contribScript{scriptName, make(map[string][]byte)}
	for _, node := range gt.Tree {
		sname, fname := node.split()
		if sname == scriptName {
			script.Files[fname] = downloadFile(scriptName, fname)
		}
	}
	if script.Files["README.md"] == nil {
		log.Fatalf("CORRUPT REPOSITORY: No README.md file found for %s.",
			scriptName)
	}
	if script.source() == nil {
		log.Fatalf("CORRUPT REPOSITORY: No script file found for %s.",
			scriptName)
	}
	return script
}

// downloadFile retrieves a single file with the given name for the given
// script. If there was a problem downloading the file, the program stops
// with an error message.
func downloadFile(scriptName, fileName string) []byte {
	url := fmt.Sprintf("%s/%s/%s", rawPrefix, scriptName, fileName)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Could not download %s: %s", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Could not read response %s: %s", url, err)
		}
		return contents
	}
	return nil
}

// configName returns the base name of the configuration file for a script.
func configName(scriptName string) string {
	return fmt.Sprintf("%s.cfg", scriptName)
}
