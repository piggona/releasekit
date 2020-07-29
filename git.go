package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/piggona/releasekit/utils"
)

func InitRepo(username, token, repo string) (r *git.Repository) {
	var (
		err error
	)
	files, _ := ioutil.ReadDir(".")
	if len(files) == 0 {
		r, err = git.PlainClone("./", false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: username,
				Password: token,
			},
			URL:      repo,
			Progress: os.Stdout,
		})
		utils.CheckIfError(err)
	} else {
		var w *git.Worktree
		r, err = git.PlainOpen("./")
		utils.CheckIfError(err)
		w, err = r.Worktree()
		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil {
			utils.Warning("pull：%s\n", err)
		}
	}
	return r
}

func CommitRepo(r *git.Repository, sig *object.Signature, comment string) error {
	var (
		w          *git.Worktree
		commitHash plumbing.Hash
		status     git.Status
		err        error
	)
	w, err = r.Worktree()
	if err != nil {
		log.Printf("get worktree error: %s\n", err)
		return err
	}
	_, err = w.Add(".")
	if err != nil {
		log.Printf("add files to repository error: %s\n", err)
		return err
	}

	status, err = w.Status()
	if err != nil {
		log.Printf("get status error: %s\n", err)
		return err
	}
	fmt.Println(status)

	commitHash, err = w.Commit(comment, &git.CommitOptions{
		Author: sig,
	})
	if err != nil {
		log.Printf("commit error: %s\n", err)
		return err
	}
	utils.Info("git show -s")
	obj, err := r.CommitObject(commitHash)
	if err != nil {
		log.Printf("get commit object error: %s\n", err)
		return err
	}
	fmt.Println(obj)
	return nil
}

func SimplePush(r *git.Repository, username, token string) error {
	auth := &http.BasicAuth{
		Username: username,
		Password: token,
	}

	po := &git.PushOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
		Auth:       auth,
	}
	utils.Info("git push --tags")
	err := r.Push(po)
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			log.Print("origin remote was up to date,no push done")
			return nil
		}
		log.Printf("push to remote origin error: %s", err)
		return err
	}
	return nil
}

func PushRepo(r *git.Repository, tag, username, token string, sig *object.Signature) error {
	_, err := setTag(r, "v"+tag, sig)
	if err != nil {
		return err
	}
	err = pushTags(r, username, token)
	if err != nil {
		return err
	}
	return nil
}

func tagExists(tag string, r *git.Repository) bool {
	tagFoundErr := "tag was found"
	utils.Info("git show-ref --tag")
	tags, err := r.TagObjects()
	if err != nil {
		log.Printf("get tags error: %s", err)
		return false
	}
	res := false
	err = tags.ForEach(func(t *object.Tag) error {
		if t.Name == tag {
			res = true
			return fmt.Errorf(tagFoundErr)
		}
		return nil
	})
	if err != nil && err.Error() != tagFoundErr {
		log.Printf("iterate tags error: %s", err)
		return false
	}
	return res
}

func setTag(r *git.Repository, tag string, sig *object.Signature) (bool, error) {
	if tagExists(tag, r) {
		log.Printf("tag %s already exists", tag)
		return false, nil
	}
	log.Printf("Set tag %s", tag)
	h, err := r.Head()
	if err != nil {
		log.Printf("get HEAD error: %s", err)
		return false, fmt.Errorf("get HEAD error: %s", err)
	}
	utils.Info("git tag -a %s %s -m \"%s\"", tag, h.Hash(), tag)
	_, err = r.CreateTag(tag, h.Hash(), &git.CreateTagOptions{
		Tagger:  sig,
		Message: tag,
	})
	if err != nil {
		log.Printf("create tag error: %s", err)
		return false, err
	}
	return true, nil
}

func pushTags(r *git.Repository, username, token string) error {
	auth := &http.BasicAuth{
		Username: username,
		Password: token,
	}

	po := &git.PushOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
		RefSpecs:   []config.RefSpec{config.RefSpec("refs/tags/*:refs/tags/*")},
		Auth:       auth,
	}
	utils.Info("git push --tags")
	err := r.Push(po)
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			log.Print("origin remote was up to date,no push done")
			return nil
		}
		log.Printf("push to remote origin error: %s", err)
		return err
	}
	return nil
}
