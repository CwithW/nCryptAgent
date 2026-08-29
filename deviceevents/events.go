package deviceevents

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	//dbt.h - https://github.com/tpn/winsdk-10/blob/master/Include/10.0.16299.0/um/Dbt.h
	DBT_DEVICEARRIVAL           = 0x8000
	DBT_DEVICEQUERYREMOVE       = 0x8001 // wants to remove, may fail
	DBT_DEVICEQUERYREMOVEFAILED = 0x8002 // removal aborted
	DBT_DEVICEREMOVEPENDING     = 0x8003 // about to remove, still avail.
	DBT_DEVICEREMOVECOMPLETE    = 0x8004 // device is gone
	DBT_DEVICETYPESPECIFIC      = 0x8005 // type specific event

	DEVICE_NOTIFY_WINDOW_HANDLE         = 0x0
	DEVICE_NOTIFY_ALL_INTERFACE_CLASSES = 0x00000004
	DBT_DEVTYP_VOLUME                   = 0x00000002 // logical volume
	DBT_DEVTYP_DEVICEINTERFACE          = 0x00000005 // device interface class

	deviceBroadcastHeaderSize          = 12
	deviceBroadcastDeviceInterfaceSize = 28
	maxDeviceBroadcastSize             = 64 * 1024
)

var (
	user32                         = windows.MustLoadDLL("user32.dll")
	registerDeviceNotificationProc = user32.MustFindProc("RegisterDeviceNotificationW")

	SMARTCARD_DEVICE_CLASS = windows.GUID{
		0xDEEBE6AD,
		0x9E01,
		0x47E2,
		[8]byte{0xA3, 0xB2, 0xA6, 0x6A, 0xA2, 0xC0, 0x36, 0xC9},
	}
)

func RegisterDeviceNotification(hwnd windows.HWND) error {

	var notificationFilter struct {
		dwSize       uint32
		dwDeviceType uint32
		dwReserved   uint32
		classGuid    windows.GUID
		szName       uint16
	}
	notificationFilter.dwSize = uint32(unsafe.Sizeof(notificationFilter))
	notificationFilter.dwDeviceType = DBT_DEVTYP_DEVICEINTERFACE
	notificationFilter.dwReserved = 0
	//notificationFilter.classGuid = SMARTCARD_DEVICE_CLASS // seems to be ignored
	notificationFilter.szName = 0

	r1, _, err := registerDeviceNotificationProc.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&notificationFilter)),
		DEVICE_NOTIFY_WINDOW_HANDLE|DEVICE_NOTIFY_ALL_INTERFACE_CLASSES,
	)
	if r1 == 0 {
		return err
	}
	return nil
}

func ReadDeviceInfo(devInfoPtr uintptr) (uint32, windows.GUID, string, error) {
	if devInfoPtr == 0 {
		return 0, windows.GUID{}, "", fmt.Errorf("device information pointer is nil")
	}

	header := unsafe.Slice((*byte)(unsafe.Pointer(devInfoPtr)), deviceBroadcastHeaderSize)
	declaredSize := binary.LittleEndian.Uint32(header[0:4])
	deviceType := binary.LittleEndian.Uint32(header[4:8])

	if declaredSize < deviceBroadcastHeaderSize {
		return 0, windows.GUID{}, "", fmt.Errorf("device information size %d is smaller than the header", declaredSize)
	}

	// WM_DEVICECHANGE broadcasts use different structures for each device type.
	// Only DEV_BROADCAST_DEVICEINTERFACE contains a class GUID and device name.
	if deviceType != DBT_DEVTYP_DEVICEINTERFACE {
		return deviceType, windows.GUID{}, "", nil
	}

	if declaredSize > maxDeviceBroadcastSize {
		return 0, windows.GUID{}, "", fmt.Errorf("device information size %d exceeds the maximum supported size", declaredSize)
	}

	deviceInfo := unsafe.Slice((*byte)(unsafe.Pointer(devInfoPtr)), int(declaredSize))
	return parseDeviceInfo(deviceInfo)
}

func parseDeviceInfo(deviceInfo []byte) (uint32, windows.GUID, string, error) {
	if len(deviceInfo) < deviceBroadcastHeaderSize {
		return 0, windows.GUID{}, "", fmt.Errorf("device information is shorter than the header")
	}

	declaredSize := binary.LittleEndian.Uint32(deviceInfo[0:4])
	deviceType := binary.LittleEndian.Uint32(deviceInfo[4:8])
	if declaredSize < deviceBroadcastHeaderSize {
		return 0, windows.GUID{}, "", fmt.Errorf("device information size %d is smaller than the header", declaredSize)
	}
	if declaredSize > uint32(len(deviceInfo)) {
		return 0, windows.GUID{}, "", fmt.Errorf("device information size %d exceeds available data %d", declaredSize, len(deviceInfo))
	}
	if deviceType != DBT_DEVTYP_DEVICEINTERFACE {
		return deviceType, windows.GUID{}, "", nil
	}
	if declaredSize < deviceBroadcastDeviceInterfaceSize {
		return 0, windows.GUID{}, "", fmt.Errorf("device interface size %d is smaller than the fixed structure", declaredSize)
	}

	guidBytes := deviceInfo[12:deviceBroadcastDeviceInterfaceSize]
	deviceGUID := windows.GUID{
		Data1: binary.LittleEndian.Uint32(guidBytes[0:4]),
		Data2: binary.LittleEndian.Uint16(guidBytes[4:6]),
		Data3: binary.LittleEndian.Uint16(guidBytes[6:8]),
	}
	copy(deviceGUID.Data4[:], guidBytes[8:16])

	deviceName := string(deviceInfo[deviceBroadcastDeviceInterfaceSize:declaredSize])
	return deviceType, deviceGUID, deviceName, nil
}
