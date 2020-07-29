package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"

	"github.com/piggona/releasekit/utils"
)

func ReleaseExec(workdir string, token string, fingerprint string) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	_, err := os.Stat(workdir + "dist/")
	if err == nil {
		err = os.RemoveAll(workdir + "dist/")
		if err != nil {
			log.Printf("remove dist error: %s\n", err)
			return err
		}
	}
	cmd := exec.Command("goreleaser", "release", "--rm-dist")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GITHUB_TOKEN="+token, "GPG_FINGERPRINT="+fingerprint)
	err = cmd.Run()
	if err != nil {
		log.Printf("cmd goreleaser error: %s\nstderr log: %s\n", err, stderr.String())
		return err
	}
	utils.Info("command goreleaser release --rm-dist output:\n")
	log.Printf("%s\n", out.String())
	return nil
}

func RunTidy(workdir string) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("cmd go mod tidy error: %s\nstderr log: %s\n", err, stderr.String())
		return err
	}
	utils.Info("command go mod tidy:\n")
	log.Printf("%s\n", out.String())
	return nil
}
