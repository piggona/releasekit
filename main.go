package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/bitly/go-simplejson"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/piggona/releasekit/utils"
)

func main() {
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

	exists, err := utils.PathExists(path)
	if !exists || err != nil {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Fatalf("create new directory error: %s\n", err)
		}
	}
	err = os.Chdir(path)
	if err != nil {
		log.Printf("change directory error: %s\n", err)
		return
	}

	r := InitRepo(username, token, repo)

	// 将旧的版本信息改为已发布
	ver, err = ModifyChangelog("CHANGELOG.md")
	if err != nil {
		log.Printf("modify changelog error: %s\n", err)
		return
	}
	log.Println(ver)
	err = RunTidy("./")
	if err != nil {
		log.Printf("run go mod tidy error: %s\n", err)
		return
	}
	// 然后commit，打tag并push
	sig := &object.Signature{
		Name:  username,
		Email: email,
		When:  time.Now(),
	}
	err = CommitRepo(r, sig, "release version "+ver)
	if err != nil {
		log.Printf("commit repo error: %s\n", err)
		return
	}
	utils.CheckIfError(err)
	err = PushRepo(r, ver, username, token, sig)
	if err != nil {
		log.Printf("push repo error: %s\n", err)
		return
	}

	// 执行Release
	err = ReleaseExec("./", token, fingerprint)
	if err != nil {
		log.Printf("release execution error: %s\n", err)
		return
	}
	err = os.RemoveAll("./dist")
	if err != nil {
		log.Printf("remove dist error: %s\n", err)
		return
	}
	// 生成新的CHANGELOG UNRELEASED
	err = SetNewVersion("CHANGELOG.md", ver, 2)
	if err != nil {
		log.Printf("set new version changelog error: %s\n", err)
		return
	}
	// push上去
	err = CommitRepo(r, sig, "new version changelog")
	if err != nil {
		log.Printf("commit repo error: %s\n", err)
		return
	}
	utils.CheckIfError(err)
	err = SimplePush(r, username, token)
	if err != nil {
		log.Printf("simple push error: %s\n", err)
		return
	}

}

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

func ModifyChangelog(filename string) (string, error) {
	var (
		file       *os.File
		wfile      *os.File
		exists     bool
		err        error
		reader     *bufio.Reader
		writer     *bufio.Writer
		regex      *regexp.Regexp
		newVersion string
	)
	// 先判断有没有这个文件
	exists, err = utils.PathExists(filename)
	if !exists || err != nil {
		exists = false
		file, err = os.Create(filename)
		if err != nil {
			log.Printf("create file error %s: %s", filename, err)
			return "", fmt.Errorf("create file error %s: %s", filename, err)
		}
	}

	// 然后取以##开头的行，取数字，将后面的Unreleased改为今日日期
	if file == nil {
		file, err = os.Open(filename)
		if err != nil {
			log.Printf("open file error %s: %s", filename, err)
			return "", fmt.Errorf("open file error %s: %s", filename, err)
		}
		defer file.Close()
	}
	wfile, err = os.Create(filename + ".tmp")
	defer wfile.Close()
	if err != nil {
		log.Printf("create temp file error %s: %s", filename, err)
		return "", fmt.Errorf("create temp file error %s: %s", filename, err)
	}
	reader = bufio.NewReader(file)
	writer = bufio.NewWriter(wfile)
	regex, err = regexp.Compile("^##.*?Unreleased\\)$")
	if err != nil {
		log.Printf("compile regex error: %s", err)
		return "", fmt.Errorf("compile regex error: %s", err)
	}

	if !exists {
		var str string
		str, newVersion = LogGenerator("")
		fmt.Fprintln(writer, str)
	} else {
		for {
			bfRead, _, err := reader.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("read line in file error %s: %s", filename, err)
				return "", fmt.Errorf("read line in file error %s: %s", filename, err)
			}
			str := *(*string)(unsafe.Pointer(&bfRead))
			if regex.MatchString(str) {
				// 把str修改一下
				str, newVersion = LogGenerator(str)
			}
			// 将该行写入文件
			fmt.Fprintln(writer, str)
		}
	}
	err = writer.Flush()
	if err != nil {
		log.Printf("writer flush error %s: %s", filename+"tmp", err)
		return "", fmt.Errorf("writer flush error %s: %s", filename+"tmp", err)
	}
	err = os.Remove(filename)
	if err != nil {
		log.Printf("remove file error %s: %s", filename, err)
		return "", fmt.Errorf("remove file error %s: %s", filename, err)
	}
	err = os.Rename(filename+".tmp", filename)
	if err != nil {
		log.Printf("rename file error %s: %s", filename+".tmp", err)
		return "", fmt.Errorf("rename file error %s: %s", filename+".tmp", err)
	}
	return newVersion, nil
}

func LogGenerator(version string) (string, string) {
	var newVersion string
	var newDate string
	if len(version) == 0 {
		newVersion = "1.0.0"
		return fmt.Sprintf("## %s (Unreleased)", newVersion), newVersion
	}
	reg, _ := regexp.Compile("[0-9]*\\.[0-9]*\\.[0-9]*")
	ver := reg.Find([]byte(version))
	now := time.Now()
	newDate = fmt.Sprintf("%s %s, %s", now.Month().String(), strconv.Itoa(now.Day()), strconv.Itoa(now.Year()))

	return fmt.Sprintf("## %s (%s)", string(ver), newDate), string(ver)
}

func SetNewVersion(filename string, ver string, mode int) error {
	strs := strings.Split(*(*string)(unsafe.Pointer(&ver)), ".")
	n, _ := strconv.Atoi(strs[mode])
	n++
	strs[mode] = strconv.Itoa(n)
	newVersion := strings.Join(strs, ".")

	f, err := os.Open(filename)
	if err != nil {
		log.Printf("open file %s error: %s\n", filename, err)
		return err
	}
	defer f.Close()
	contents, _ := ioutil.ReadAll(f)
	newContents := fmt.Sprintf("## %s (Unreleased)\n", newVersion) + *(*string)(unsafe.Pointer(&contents))
	wf, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("open write file %s error: %s\n", filename, err)
		return err
	}
	num, err := wf.WriteString(newContents)
	if err != nil || num < 1 {
		log.Printf("write file error: %v\n,wrote %d bytes", err, num)
		return fmt.Errorf("write error %s", err)
	}
	return nil
}

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
