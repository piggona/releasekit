package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/piggona/releasekit/utils"
)

const (
	MAINVER = iota
	SUBVER
	STAGEVER
)

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
