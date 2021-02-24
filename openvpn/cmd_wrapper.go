/*
 * go-openvpn -- Go gettable library for wrapping Openvpn functionality in go way.
 *
 * Copyright (C) 2020 BlockDev AG.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License Version 3
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.

 * You should have received a copy of the GNU Affero General Public License
 * along with this program in the COPYING file.
 * If not, see <http://www.gnu.org/licenses/>.
 */

package openvpn

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"sync"

	"github.com/trevor403/go-openvpn-static/openvpn/log"
)

// CommandFunc represents the func for running external commands
type CommandFunc func(arg ...string) *exec.Cmd

// NewCmdWrapper returns process wrapper for given executable
func NewCmdWrapper(logPrefix string, commandFunc CommandFunc) *CmdWrapper {
	return &CmdWrapper{
		command:            commandFunc,
		logPrefix:          logPrefix,
		CmdExitError:       make(chan error, 1), //channel should have capacity to hold single process exit error
		cmdShutdownStarted: make(chan bool),
		cmdShutdownWaiter:  sync.WaitGroup{},
	}
}

// CmdWrapper struct defines process wrapper which handles clean shutdown, tracks executable exit errors, logs stdout and stderr to logger
type CmdWrapper struct {
	command            CommandFunc
	logPrefix          string
	CmdExitError       chan error
	cmdShutdownStarted chan bool
	cmdShutdownWaiter  sync.WaitGroup
	closesOnce         sync.Once
}

// Start underlying binary defined by process wrapper with given arguments
func (cw *CmdWrapper) Start(arguments []string) (err error) {
	// Create the command
	cmd := cw.command(arguments...)

	if len(cmd.Args) == 0 {
		return errors.New("nothing to execute for an empty command")
	}

	log.Info(cw.logPrefix, "Starting cmd:", cmd.Args[0], "with arguments:", arguments)

	// Attach logger for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go cw.outputToLog(stdout, "Stdout:")
	go cw.outputToLog(stderr, "Stderr:")

	// Try to start the cmd
	err = cmd.Start()
	if err != nil {
		return err
	}

	// Watch if the cmd exits
	go cw.waitForExit(cmd)

	cw.cmdShutdownWaiter.Add(1)
	go func() {
		cw.waitForShutdown(cmd)
		defer cw.cmdShutdownWaiter.Done()
	}()

	return
}

// Wait function wait until executable exits and then returns exit error reported by executable
func (cw *CmdWrapper) Wait() error {
	return <-cw.CmdExitError
}

// Stop function stops (or sends request to stop) underlying executable and waits until stdout/stderr and shutdown monitors are finished
func (cw *CmdWrapper) Stop() {
	cw.closesOnce.Do(func() {
		close(cw.cmdShutdownStarted)
	})
	cw.cmdShutdownWaiter.Wait()
}

func (cw *CmdWrapper) outputToLog(output io.ReadCloser, streamPrefix string) {
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		log.Debug(cw.logPrefix, streamPrefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Warn(cw.logPrefix, streamPrefix, "failed to read:", err)
	} else {
		log.Info(cw.logPrefix, streamPrefix, "stream ended")
	}
}

func (cw *CmdWrapper) waitForExit(cmd *exec.Cmd) {
	err := cmd.Wait()
	cw.CmdExitError <- err
	close(cw.CmdExitError)
}

func (cw *CmdWrapper) waitForShutdown(cmd *exec.Cmd) {
	<-cw.cmdShutdownStarted
	//First - shutdown gracefully by sending SIGINT (the only two signals guaranteed to be present in all OS'es SIGINT and SIGKILL)
	//TODO - add timer and send SIGKILL after timeout?
	if err := cmd.Process.Signal(exitSignal); err != nil {
		log.Error(cw.logPrefix, "Error killing cw:", err)
	}

	// Wait for command to quit
	<-cw.CmdExitError
}
