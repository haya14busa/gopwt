package main

import (
	"flag"
	"fmt"
	"github.com/mattn/go-isatty"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	termw    = 0
	verbose  = flag.Bool("v", false, "This will be passed to `go test`")
	testdata = flag.String("testdata", "testdata", "name of test data directories. e.g. -testdata testdata,migrations")
)

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
		return
	}
}
func doMain() error {
	if runtime.Version() == "go1.4" {
		return fmt.Errorf("go1.4 is not supported. please bump to go1.4.1 or later")
	}

	flag.Parse()

	if isatty.IsTerminal(os.Stdout.Fd()) {
		termw = getTermCols(os.Stdin.Fd())
	}

	tempGoPath, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempGoPath)

	root := flag.Arg(0)

	pkgInfo, err := newPackageInfo(root)
	if err != nil {
		return err
	}

	err = rewrite(tempGoPath, pkgInfo)
	if err != nil {
		return err
	}

	err = runTest(tempGoPath, pkgInfo, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	return nil
}

func rewrite(tempGoPath string, pkgInfo *packageInfo) error {
	tempGoSrcDir := filepath.Join(tempGoPath, "src")

	err := filepath.Walk(pkgInfo.dirPath, func(path string, fInfo os.FileInfo, err error) error {
		if fInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		if !fInfo.IsDir() {
			return nil
		}

		files, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		if !containsGoFile(files) {
			// sub-packages maybe have gofiles, even if itself don't has gofiles
			if containsDirectory(files) {
				return nil
			}
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(pkgInfo.dirPath, path)
		if err != nil {
			return err
		}

		for _, tdata := range strings.Split(*testdata, ",") {
			if strings.Split(rel, "/")[0] == tdata {
				return filepath.SkipDir
			}
		}

		if rel != "." {
			if filepath.HasPrefix(rel, ".") {
				return filepath.SkipDir
			}

			if !pkgInfo.recursive {
				return filepath.SkipDir
			}
		}

		importPath := filepath.Join(pkgInfo.importPath, rel)

		err = os.MkdirAll(filepath.Join(tempGoSrcDir, importPath), os.ModePerm)
		if err != nil {
			return err
		}

		err = rewritePackage(path, importPath, tempGoSrcDir)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func runTest(goPath string, pkgInfo *packageInfo, stdout, stderr io.Writer) error {
	err := os.Setenv("GOPATH", goPath+":"+os.Getenv("GOPATH"))
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "test")
	if *verbose {
		cmd.Args = append(cmd.Args, "-v")
	}
	cmd.Args = append(cmd.Args, pkgInfo.ToGoTestArg())
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
