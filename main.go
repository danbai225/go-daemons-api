package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//https://www.cnblogs.com/tobycnblogs/p/9981796.html
var daemon bool

func init() {
	flag.BoolVar(&daemon, "d", false, "是否为守护启动模式")
	flag.Parse()
	if os.Getppid() != 1 && daemon && len(os.Args) >= 2 {
		arg := make([]string, 0)
		if len(os.Args) > 3 {
			arg = os.Args[3:]
		}
		cmdStr := os.Args[2]
		cmdStr += strings.Join(arg, "")
		cmd := exec.Command(os.Args[2], arg...)
		stat, err := os.Stat("pid/")
		if err != nil || !stat.IsDir() {
			os.Mkdir("pid/", 0777)
		}
		cmd.Start()
		syscall.Umask(27)
		pidFile := fmt.Sprintf("pid%c%s.pid", os.PathSeparator, cmdStr)
		file, err := os.OpenFile(pidFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		defer file.Close()
		if err == nil {
			file.WriteString(strconv.Itoa(cmd.Process.Pid))
		}
		os.Exit(0)
	}
	//go checkPid()
}
func checkPid() {
	for {
		rd, err := ioutil.ReadDir("pid/")
		if err == nil {
			for _, fi := range rd {
				if !fi.IsDir() {
					fmt.Println(fi.Name())
				}
			}
		}
		time.Sleep(time.Second)
	}
}
func main() {
	router := gin.Default()
	router.POST("/run", run)
	router.Run(":8080")
}
func run(c *gin.Context) {
	a := struct {
		Cmd string `json:"cmd"`
	}{}
	if err := c.ShouldBindJSON(&a); err != nil {
		c.AbortWithStatusJSON(
			http.StatusOK,
			gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "ok", "data": daemonRun(a.Cmd)})
	return
}

func daemonRun(cmd string) uint {
	split := strings.Split(cmd, " ")
	split = append([]string{ExecPath(), "-d"}, split...)
	attr := &os.ProcAttr{
		Dir: "./",
		Env: os.Environ(),
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
		Sys: &syscall.SysProcAttr{
			//Chroot:     d.Chroot,
			Setsid: true,
		},
	}
	if c, err := os.StartProcess(ExecPath(), split, attr); err == nil {
		defer c.Release()
		pidFile := fmt.Sprintf("pid%c%s.pid", os.PathSeparator, strings.ReplaceAll(cmd, " ", ""))
		time.Sleep(time.Second)
		bytes, err := ioutil.ReadFile(pidFile)
		if err == nil {
			parseUint, _ := strconv.ParseUint(string(bytes), 10, 32)
			return uint(parseUint)
		}
	}
	return 0
}

var appPath = ""

func ExecPath() string {
	if appPath == "" {
		file, err := exec.LookPath(os.Args[0])
		if err != nil {
			return ""
		}
		appPath, _ = filepath.Abs(file)
	}
	return appPath
}
