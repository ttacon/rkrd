package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// working name rkrd

func main() {
	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name:  "cleanup",
			Usage: "delete all rkrdr files",
			Action: func(c *cli.Context) error {
				return cleanupRkrdrFiles()
			},
		},
		{
			Name:    "diff",
			Aliases: []string{"d"},
			Usage:   "diff two rkrdr diffs",
			Action: func(c *cli.Context) error {
				args := c.Args()
				if len(args) != 2 {
					logrus.Error("must provide two rkrdr files to diff")
					os.Exit(1)
				}

				return diff(args[0], args[1])
			},
		},
		{
			Name:    "proxy",
			Aliases: []string{"p"},
			Usage:   "run rkrd proxy",
			Action: func(c *cli.Context) error {
				runProxy("8080")
				return nil
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func runProxy(listeningPort string) {
	if len(listeningPort) == 0 {
		logrus.Error("must provide listening port")
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%s", listeningPort)
	rkrd := NewRkrd(addr)
	if err := rkrd.Start(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	logrus.Info("up and running")

	for {
		if err := rkrd.HandleConnection(); err != nil {
			logrus.Error(err)
		}
	}

	v := make(chan struct{})
	<-v
}

func diff(file0, file1 string) error {
	data0, err := ioutil.ReadFile(file0)
	if err != nil {
		return err
	}

	data1, err := ioutil.ReadFile(file1)
	if err != nil {
		return err
	}

	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(string(data0), string(data1), false)

	if len(diffs) > 1 ||
		(len(diffs) == 1 && diffs[0].Type != diffmatchpatch.DiffEqual) {
		fmt.Println(dmp.DiffPrettyText(diffs))
		os.Exit(1)
	}
	return nil
}

func cleanupRkrdrFiles() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(wd)
	if err != nil {
		return err
	}

	// Let's be pretty safe here.
	rkrdrFileNameRegexp := regexp.MustCompile("rkrdr-(.*)\\.rkrdr")

	for _, file := range files {
		match := rkrdrFileNameRegexp.FindStringSubmatch(file.Name())
		if len(match) != 2 {
			// Not a match, keep looking.
			continue
		}

		if _, err := strconv.Atoi(match[1]); err != nil {
			// Not a valid digit so skip it.
			continue
		}

		// If we get here, for now, we assume that this is a valid
		// rkrdr file.
		if err := os.Remove(file.Name()); err != nil {
			return err
		}
	}

	return nil
}
