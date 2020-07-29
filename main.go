package main

import (
	"log"
	"os"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/piggona/releasekit/utils"
)

func main() {

	releaser()

}

func releaser() {
	// 1. 读取配置文件
	var (
		err  error
		conf *simplejson.Json
		ver  string
	)
	confpath := "./config.json"
	path := "./repo/"
	conf, err = utils.ReadJSON(confpath)
	if err != nil {
		log.Fatalf("read config file error: %s\n", err)
	}
	username, _ := conf.Get("username").String()
	email, _ := conf.Get("email").String()
	token, _ := conf.Get("accesstoken").String()
	repo, _ := conf.Get("git_repo").String()
	fingerprint, _ := conf.Get("gpg_fingerprint").String()

	// 2. 创建工作区repo
	exists, err := utils.PathExists(path)
	if !exists || err != nil {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Fatalf("create new directory error: %s\n", err)
		}
	}
	err = os.Chdir(path)
	if err != nil {
		log.Fatalf("change directory error: %s\n", err)
	}
	r := InitRepo(username, token, repo)

	// 3. 将旧版本信息改为已发布
	ver, err = ModifyChangelog("CHANGELOG.md")
	if err != nil {
		log.Fatalf("modify changelog error: %s\n", err)
	}
	log.Println(ver)
	// go mod tidy check
	err = RunTidy("./")
	if err != nil {
		log.Fatalf("run go mod tidy error: %s\n", err)
	}
	// 然后commit，打tag并push
	sig := &object.Signature{
		Name:  username,
		Email: email,
		When:  time.Now(),
	}
	err = CommitRepo(r, sig, "release version "+ver)
	if err != nil {
		log.Fatalf("commit repo error: %s\n", err)
	}
	utils.CheckIfError(err)
	err = PushRepo(r, ver, username, token, sig)
	if err != nil {
		log.Fatalf("push repo error: %s\n", err)
	}

	// 4. 执行goreleaser发布版本
	err = ReleaseExec("./", token, fingerprint)
	if err != nil {
		log.Fatalf("release execution error: %s\n", err)
	}
	err = os.RemoveAll("./dist")
	if err != nil {
		log.Fatalf("remove dist error: %s\n", err)
	}

	// 5. 生成新的CHANGELOG UNRELEASED
	err = SetNewVersion("CHANGELOG.md", ver, STAGEVER)
	if err != nil {
		log.Fatalf("set new version changelog error: %s\n", err)
	}
	// push上去
	err = CommitRepo(r, sig, "new version changelog")
	if err != nil {
		log.Fatalf("commit repo error: %s\n", err)
	}
	utils.CheckIfError(err)
	err = SimplePush(r, username, token)
	if err != nil {
		log.Fatalf("simple push error: %s\n", err)
	}
}
