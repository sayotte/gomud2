package telnet

import (
	"bytes"
	"testing"
)

func Test_findLines(t *testing.T) {
	testCases := map[string]struct {
		inBytes          []byte
		expectedOutLines [][]byte
		expectedConsumed int
	}{
		"single line CRLF": {
			inBytes:          []byte("1234\r\n"),
			expectedOutLines: [][]byte{[]byte("1234")},
			expectedConsumed: 6,
		},
		"single line CR-null": {
			inBytes:          []byte("1234\r\x00"),
			expectedOutLines: [][]byte{[]byte("1234")},
			expectedConsumed: 6,
		},
		"single line \\n": {
			inBytes:          []byte("1234\n"),
			expectedOutLines: [][]byte{[]byte("1234")},
			expectedConsumed: 5,
		},
		"multi-line CRLF, with trailing data": {
			inBytes: []byte("1234\r\n5678\r\n90"),
			expectedOutLines: [][]byte{
				[]byte("1234"),
				[]byte("5678"),
			},
			expectedConsumed: 12,
		},
		"multi-line, mixed terminators": {
			inBytes: []byte("123\r\n456\r\x00789\n"),
			expectedOutLines: [][]byte{
				[]byte("123"),
				[]byte("456"),
				[]byte("789"),
			},
			expectedConsumed: 14,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actualLines, consumed := findLines(tc.inBytes)
			if len(actualLines) != len(tc.expectedOutLines) {
				t.Fatalf("# of found-lines (%d) != # of expected lines (%d)", len(actualLines), len(tc.expectedOutLines))
			}
			for i := range actualLines {
				if !bytes.Equal(actualLines[i], tc.expectedOutLines[i]) {
					t.Errorf("line #%d: expected %q, got %q", i, string(tc.expectedOutLines[i]), string(actualLines[i]))
				}
			}
			if consumed != tc.expectedConsumed {
				t.Errorf("expected consumed == %d, got %d", tc.expectedConsumed, consumed)
			}
		})
	}
}
