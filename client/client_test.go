package client_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ks888/hornet/client"
)

var serverAddr string

func Test_TestAction(t *testing.T) {
	testfile := "/path/to/test/file"

	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: serverAddr, TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(logger.String(), testfile) {
		t.Errorf("unexpected log: %v", logger.String())
	}
}

func TestMain(m *testing.M) {
	// can't use defer here
	os.Exit(doTestMain(m))
}

func doTestMain(m *testing.M) int {
	binaryPath, err := buildHornetd()
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(binaryPath))

	unusedPort, err := findUnusedPort()
	if err != nil {
		log.Fatal(err)
	}
	serverAddr = fmt.Sprintf("localhost:%d", unusedPort)

	cmd, err := runHornetd(binaryPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	return m.Run()
}

func buildHornetd() (string, error) {
	dir, err := ioutil.TempDir("", "hornetd")
	if err != nil {
		return "", err
	}

	binaryPath := filepath.Join(dir, "hornetd")
	out, err := exec.Command("go", "build", "-o", binaryPath, "github.com/ks888/hornet/cmd/hornetd").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build the hornetd binary: %w\nlog:\n%s", err, string(out))
	}

	return binaryPath, nil
}

func findUnusedPort() (int, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{})
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port, nil
}

func runHornetd(binaryPath string) (*exec.Cmd, error) {
	cmd := exec.Command(binaryPath, serverAddr)
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	for i := 0; i < 32; i++ {
		conn, err := net.Dial("tcp", serverAddr)
		if err == nil {
			conn.Close()
			return cmd, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, errors.New("failed to run hornetd server")
}
