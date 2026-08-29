package deviceevents

import (
	"encoding/binary"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestReadDeviceInfoIgnoresVolumeBroadcast(t *testing.T) {
	deviceInfo := make([]byte, 20)
	binary.LittleEndian.PutUint32(deviceInfo[0:4], uint32(len(deviceInfo)))
	binary.LittleEndian.PutUint32(deviceInfo[4:8], DBT_DEVTYP_VOLUME)

	deviceType, deviceGUID, deviceName, err := ReadDeviceInfo(uintptr(unsafe.Pointer(&deviceInfo[0])))
	runtime.KeepAlive(deviceInfo)
	if err != nil {
		t.Fatalf("ReadDeviceInfo() error = %v", err)
	}
	if deviceType != DBT_DEVTYP_VOLUME {
		t.Fatalf("device type = %d, want %d", deviceType, DBT_DEVTYP_VOLUME)
	}
	if deviceGUID != (windows.GUID{}) {
		t.Fatalf("device GUID = %v, want zero value", deviceGUID)
	}
	if deviceName != "" {
		t.Fatalf("device name = %q, want empty", deviceName)
	}
}

func TestParseDeviceInfoRejectsShortDeviceInterface(t *testing.T) {
	deviceInfo := make([]byte, 20)
	binary.LittleEndian.PutUint32(deviceInfo[0:4], uint32(len(deviceInfo)))
	binary.LittleEndian.PutUint32(deviceInfo[4:8], DBT_DEVTYP_DEVICEINTERFACE)

	if _, _, _, err := parseDeviceInfo(deviceInfo); err == nil {
		t.Fatal("parseDeviceInfo() error = nil, want short structure error")
	}
}

func TestParseDeviceInfoDeviceInterface(t *testing.T) {
	deviceName := []byte{'d', 0, 'e', 0, 'v', 0, 0, 0}
	deviceInfo := make([]byte, deviceBroadcastDeviceInterfaceSize+len(deviceName))
	binary.LittleEndian.PutUint32(deviceInfo[0:4], uint32(len(deviceInfo)))
	binary.LittleEndian.PutUint32(deviceInfo[4:8], DBT_DEVTYP_DEVICEINTERFACE)

	wantGUID := windows.GUID{
		Data1: 0x11223344,
		Data2: 0x5566,
		Data3: 0x7788,
		Data4: [8]byte{0x90, 0xAB, 0xCD, 0xEF, 1, 2, 3, 4},
	}
	binary.LittleEndian.PutUint32(deviceInfo[12:16], wantGUID.Data1)
	binary.LittleEndian.PutUint16(deviceInfo[16:18], wantGUID.Data2)
	binary.LittleEndian.PutUint16(deviceInfo[18:20], wantGUID.Data3)
	copy(deviceInfo[20:28], wantGUID.Data4[:])
	copy(deviceInfo[deviceBroadcastDeviceInterfaceSize:], deviceName)

	deviceType, deviceGUID, gotName, err := parseDeviceInfo(deviceInfo)
	if err != nil {
		t.Fatalf("parseDeviceInfo() error = %v", err)
	}
	if deviceType != DBT_DEVTYP_DEVICEINTERFACE {
		t.Fatalf("device type = %d, want %d", deviceType, DBT_DEVTYP_DEVICEINTERFACE)
	}
	if deviceGUID != wantGUID {
		t.Fatalf("device GUID = %v, want %v", deviceGUID, wantGUID)
	}
	if gotName != string(deviceName) {
		t.Fatalf("device name = %q, want %q", gotName, string(deviceName))
	}
}

func TestParseDeviceInfoRejectsDeclaredSizeBeyondData(t *testing.T) {
	deviceInfo := make([]byte, deviceBroadcastHeaderSize)
	binary.LittleEndian.PutUint32(deviceInfo[0:4], uint32(len(deviceInfo)+1))
	binary.LittleEndian.PutUint32(deviceInfo[4:8], DBT_DEVTYP_DEVICEINTERFACE)

	if _, _, _, err := parseDeviceInfo(deviceInfo); err == nil {
		t.Fatal("parseDeviceInfo() error = nil, want declared size error")
	}
}
