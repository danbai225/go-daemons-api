package main

import (
	"encoding/base64"
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
	"sync"
	"syscall"
	"time"
)

//https://www.cnblogs.com/tobycnblogs/p/9981796.html
var daemon bool
var launcher bool

func init() {
	flag.BoolVar(&daemon, "d", false, "是否为守护启动模式")
	flag.BoolVar(&launcher, "l", false, "Launcher")
	flag.Parse()
	if launcher {
		return
	}
	if os.Getppid() != 1 && daemon && len(os.Args) >= 2 {
		arg := make([]string, 0)
		if len(os.Args) > 3 {
			arg = os.Args[3:]
		}
		cmdStr := os.Args[2]
		cmdStr += strings.Join(arg, "")
		cmd := exec.Command(os.Args[2], arg...)
		//文件权限描述符
		syscall.Umask(27)
		cmd.Start()
		print(cmd.Process.Pid)
		os.Exit(0)
	} else if !launcher {
		//pid目录
		stat, err := os.Stat("pid/")
		if err != nil || !stat.IsDir() {
			os.Mkdir("pid/", 0777)
		}
		go checkPidRun()
	}
}

var lock sync.Mutex

func checkPidRun() {
	for {
		rd, err := ioutil.ReadDir("pid/")
		if err == nil {
			for _, fi := range rd {
				if !fi.IsDir() {
					path := fmt.Sprintf("pid%c%s", os.PathSeparator, fi.Name())
					bytes, err := ioutil.ReadFile(path)
					if err == nil {
						pid, _ := strconv.ParseInt(string(bytes), 10, 32)
						if !checkPid(int(pid)) {
							lock.Lock()
							os.Remove(path)
							lock.Unlock()
						}
					}
				}
			}
		}
		time.Sleep(time.Second)
	}
}
func main() {
	if launcher {
		if len(os.Args) >= 2 {
			u := daemonRun(os.Args[2])
			print(u)
			os.Exit(0)
		}
	} else {
		router := gin.Default()
		router.POST("/run", run)
		router.Run(":8080")
	}
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
	lock.Lock()
	defer lock.Unlock()
	command := exec.Command(ExecPath(), "-l", a.Cmd)
	out, _ := command.CombinedOutput()
	parseUint, _ := strconv.ParseUint(string(out), 10, 32)
	c.JSON(http.StatusOK, gin.H{"msg": "ok", "data": map[string]interface{}{
		"pid": parseUint,
	}})
	return
}

func Base64Encoded(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
func Base64Decoded(str string) string {
	decoded, _ := base64.StdEncoding.DecodeString(str)
	return string(decoded)
}

func daemonRun(cmd string) uint {
	split := strings.Split(cmd, " ")
	split = append([]string{ExecPath(), "-d"}, split...)
	pidFile := fmt.Sprintf("pid%c%s.pid", os.PathSeparator, Base64Encoded(cmd))
	file, _ := os.OpenFile(pidFile, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
	attr := &os.ProcAttr{
		Dir: "./",
		Env: os.Environ(),
		Files: []*os.File{
			file,
			file,
			file,
		},
		Sys: &syscall.SysProcAttr{
			//Chroot:     d.Chroot,
			Setsid: true,
		},
	}
	if c, err := os.StartProcess(ExecPath(), split, attr); err == nil {
		defer c.Release()
		c.Wait()
		file.Close()
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

// Will return true if the process with PID exists.
func checkPid(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	} else {
		return true
	}
}
