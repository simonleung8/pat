package experiments

import (
	"errors"
	"math/rand"
	"time"
	"os/exec"
	"fmt"

	"github.com/nu7hatch/gouuid"
	. "github.com/pivotal-cf-experimental/cf-test-helpers/cf"
)

//Todo(simon) Remove, for dev testing only
func random(min, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	r := min + rand.Intn(max-min)
	return r
}

func Dummy() error {
	time.Sleep(time.Duration(random(1, 5)) * time.Second)
	return nil
}

func DummyWithErrors() error {
	Dummy()
	if random(0, 10) > 8 {
		return errors.New("Random (dummy) error")
	}
	return nil
}

func Push() error {
	exec.Command("export PATH=/tmp/cache/gcf")
	out1, err1 := exec.Command("ls -a /").Output()
	fmt.Printf("ls /: %s \n", out1)
	fmt.Printf("ls err: %s \n", err1)
	out2, err2 := exec.Command("echo hihi").Output()
	fmt.Printf("echo: %s \n", out2)
	fmt.Printf(" err2: %s \n", err2)
	out3, err3 := exec.Command("export").Output()
	fmt.Printf("THe path is: %s \n", out3)
	fmt.Printf(" err3: %s \n", err3)
	err := Cf("login", "-u", "admin", "-p", "admin").ExpectOutput("OK")
	guid, _ := uuid.NewV4()
	_ = Cf("push", "pats-"+guid.String(), "patsapp", "-m", "64M", "-p", "assets/hello-world").ExpectOutput("App started")
	return err
}
