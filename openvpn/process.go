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
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/trevor403/go-openvpn-static/openvpn/config"
	"github.com/trevor403/go-openvpn-static/openvpn/management"
	"github.com/trevor403/go-openvpn-static/openvpn/tunnel"
)

const openvpnManagementLogPrefix = "[openvpn-mgmt]"
const openvpnProcessLogPrefix = "[openvpn-proc]"

// OpenvpnProcess represents an openvpn process manager
type OpenvpnProcess struct {
	config      *config.GenericConfig
	tunnelSetup tunnel.Setup
	management  *management.Management
	cmd         *CmdWrapper
}

func newProcess(
	tunnelSetup tunnel.Setup,
	config *config.GenericConfig,
	execCommand func(arg ...string) *exec.Cmd,
	middlewares ...management.Middleware,
) *OpenvpnProcess {
	return &OpenvpnProcess{
		tunnelSetup: tunnelSetup,
		config:      config,
		management:  management.NewManagement(management.LocalhostOnRandomPort, openvpnManagementLogPrefix, middlewares...),
		cmd:         NewCmdWrapper(openvpnProcessLogPrefix, execCommand),
	}
}

// Start starts the openvpn process
func (openvpn *OpenvpnProcess) Start() error {
	if err := openvpn.tunnelSetup.Setup(openvpn.config); err != nil {
		return err
	}

	err := openvpn.management.WaitForConnection()
	if err != nil {
		openvpn.tunnelSetup.Stop()
		return err
	}

	addr := openvpn.management.BoundAddress
	openvpn.config.SetManagementAddress(addr.IP, addr.Port)

	// Fetch the current arguments
	arguments, err := (*openvpn.config).ToArguments()
	if err != nil {
		openvpn.management.Stop()
		openvpn.tunnelSetup.Stop()
		return err
	}

	//nil returned from process.Start doesn't guarantee that openvpn itself initialized correctly and accepted all arguments
	//it simply means that OS started process with specified args
	err = openvpn.cmd.Start(arguments)
	if err != nil {
		openvpn.management.Stop()
		openvpn.tunnelSetup.Stop()
		return err
	}

	select {
	case connAccepted := <-openvpn.management.Connected:
		if connAccepted {
			return nil
		}
		return errors.New("management failed to accept connection")
	case exitError := <-openvpn.cmd.CmdExitError:
		openvpn.management.Stop()
		openvpn.tunnelSetup.Stop()
		if exitError != nil {
			return exitError
		}
		return errors.New("openvpn process died too early")
	case <-time.After(2 * time.Second):
		return errors.New("management connection wait timeout")
	}
}

// Wait waits for the openvpn process to complete
func (openvpn *OpenvpnProcess) Wait() error {
	return openvpn.cmd.Wait()
}

// Stop stops the openvpn process
func (openvpn *OpenvpnProcess) Stop() {
	waiter := sync.WaitGroup{}
	//TODO which to signal for close first ?
	//if we stop process before management, managemnt won't have a chance to send any commands from middlewares on stop
	//if we stop management first - it will miss important EXITING state from process
	waiter.Add(1)
	go func() {
		defer waiter.Done()
		openvpn.cmd.Stop()
	}()

	waiter.Add(1)
	go func() {
		defer waiter.Done()
		openvpn.management.Stop()
	}()
	waiter.Wait()

	openvpn.tunnelSetup.Stop()
}

// DeviceName returns tunnel device name
func (openvpn *OpenvpnProcess) DeviceName() string {
	return openvpn.tunnelSetup.DeviceName()
}
