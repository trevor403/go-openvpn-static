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

package bytescount

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/trevor403/go-openvpn-static/openvpn/management"
)

func Test_Factory(t *testing.T) {
	statsRecorder := fakeStatsRecorder{}
	middleware := NewMiddleware(statsRecorder.record, 1*time.Second)
	assert.NotNil(t, middleware)
}

func Test_Start(t *testing.T) {
	statsRecorder := fakeStatsRecorder{}
	middleware := NewMiddleware(statsRecorder.record, 1*time.Second)
	mockConnection := &management.MockConnection{}
	middleware.Start(mockConnection)
	assert.Equal(t, "bytecount 1", mockConnection.LastLine)
}

func Test_Stop(t *testing.T) {
	statsRecorder := fakeStatsRecorder{}
	middleware := NewMiddleware(statsRecorder.record, 1*time.Second)
	mockConnection := &management.MockConnection{}
	middleware.Stop(mockConnection)
	assert.Equal(t, "bytecount 0", mockConnection.LastLine)
}

func Test_ConsumeLine(t *testing.T) {
	var tests = []struct {
		line                  string
		expectedConsumed      bool
		expectedError         error
		expectedBytesReceived uint64
		expectedBytesSent     uint64
	}{
		{">BYTECOUNT:3018,3264", true, nil, 3018, 3264},
		{">BYTECOUNT:0,3264", true, nil, 0, 3264},
		{">BYTECOUNT:3018,", true, errors.New(`strconv.ParseInt: parsing "": invalid syntax`), 0, 0},
		{">BYTECOUNT:,", true, errors.New(`strconv.ParseInt: parsing "": invalid syntax`), 0, 0},
		{"OTHER", false, nil, 0, 0},
		{"BYTECOUNT", false, nil, 0, 0},
		{"BYTECOUNT:", false, nil, 0, 0},
		{"BYTECOUNT:3018,3264", false, nil, 0, 0},
		{">BYTECOUNTT:3018,3264", false, nil, 0, 0},
	}

	for _, test := range tests {
		statsRecorder := &fakeStatsRecorder{}
		middleware := NewMiddleware(statsRecorder.record, 1*time.Second)
		consumed, err := middleware.ConsumeLine(test.line)
		if test.expectedError != nil {
			assert.Error(t, test.expectedError, err.Error(), test.line)
		} else {
			assert.NoError(t, err, test.line)
		}
		assert.Equal(t, test.expectedConsumed, consumed, test.line)
		assert.Equal(t, test.expectedBytesReceived, statsRecorder.BytesIn)
		assert.Equal(t, test.expectedBytesSent, statsRecorder.BytesOut)
	}
}
