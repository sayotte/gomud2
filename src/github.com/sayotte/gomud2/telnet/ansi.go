package telnet

var (
	ansiCSI = []byte("\033[") // ^[, CSI == "control sequence introducer"

	// cursor / buffer manipulation
	ansiBufferErase   = append(ansiCSI, []byte("2J")...)
	ansiCursorTopLeft = append(ansiCSI, byte('H'))
)
