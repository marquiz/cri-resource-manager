// Copyright 2021 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sysfs

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	logger "github.com/intel/cri-resource-manager/pkg/log"
)

type SstPackageInfo struct {
	// Gereric PP info
	PPSupported    bool
	PPLocked       bool
	PPVersion      int
	PPCurrentLevel int
	PPMaxLevel     int

	// Information about the currently active PP level
	CPSupported bool
	CPEnabled   bool
	CPPriority  CPPriorityType
	BFSupported bool
	BFEnabled   bool
	BFCores     IDSet
	TFSupported bool
	TFEnabled   bool

	ClosInfo [NumClos]SstClosInfo
}

const NumClos = 4

type SstClosInfo struct {
	EPP                  int
	ProportionalPriority int
	MinFreq              int
	MaxFreq              int
	DesiredFreq          int
}

type CPPriorityType int

const (
	Proportional CPPriorityType = 0
	Ordered      CPPriorityType = 1
)

const isstDevPath = "/dev/isst_interface"

func SstSupported() bool {
	if _, err := os.Stat(isstDevPath); err != nil {
		if !os.IsNotExist(err) {
			log.Warn("failed to access sst device %q: %v", isstDevPath, err)
		} else {
			log.Debug("sst device %q does not exist", isstDevPath)
		}
		return false
	}
	return true
}

func getSstPackageInfo(pkg *cpuPackage) (SstPackageInfo, error) {
	info := SstPackageInfo{}
	ids := pkg.cpus.SortedMembers()
	cpu := ids[0] // We just need to pass one logical cpu from the pkg as an arg

	var rsp uint32
	var err error

	// Read perf-profile feature info
	if rsp, err = sendMboxCmd(cpu, CONFIG_TDP, CONFIG_TDP_GET_LEVELS_INFO, 0); err != nil {
		return info, fmt.Errorf("failed to read SST PP info: %v", err)
	}
	info.PPSupported = getBits(rsp, 31, 31) != 0
	info.PPLocked = getBits(rsp, 24, 24) != 0
	info.PPCurrentLevel = int(getBits(rsp, 16, 23))
	info.PPMaxLevel = int(getBits(rsp, 8, 15))
	info.PPVersion = int(getBits(rsp, 0, 7))

	// Forget about older hw with partial/convoluted support
	if info.PPVersion < 3 {
		log.Info("SST PP version %d (less than 3), giving up...")
		return info, nil
	}

	// Read the status of currently active perf-profile
	if info.PPSupported {
		if rsp, err = sendMboxCmd(cpu, CONFIG_TDP, CONFIG_TDP_GET_TDP_CONTROL, uint32(info.PPCurrentLevel)); err != nil {
			return info, fmt.Errorf("failed to read SST CP status: %v", err)
		}

		info.BFSupported = isBitSet(rsp, 1)
		info.BFEnabled = isBitSet(rsp, 17)

		info.TFSupported = isBitSet(rsp, 0)
		info.TFEnabled = isBitSet(rsp, 16)

	}

	// Read base-frequency info
	if info.BFSupported {
		info.BFCores = IDSet{}

		punitCoreIDs := make(map[ID]IDSet, len(ids))
		var maxPunitCore ID
		for _, id := range ids {
			pc, err := punitCPU(id)
			if err != nil {
				return info, err
			}
			punitCore := pc >> 1
			if _, ok := punitCoreIDs[punitCore]; !ok {
				punitCoreIDs[punitCore] = IDSet{}
			}
			punitCoreIDs[punitCore].Add(id)
			if punitCore > maxPunitCore {
				maxPunitCore = punitCore
			}
		}

		// Read out core masks in batches of 32 (32 bits per response)
		for i := 0; i <= int(maxPunitCore)/32; i++ {
			if rsp, err = sendMboxCmd(cpu, CONFIG_TDP, CONFIG_TDP_PBF_GET_CORE_MASK_INFO, uint32(info.PPCurrentLevel+(i<<8))); err != nil {
				return info, fmt.Errorf("failed to read SST BF core mask (#%d): %v", i, err)
			}
			for bit := 0; bit < 32; bit++ {
				if isBitSet(rsp, uint32(bit)) {
					info.BFCores.Add(punitCoreIDs[ID(i*32+bit)].Members()...)
				}
			}
		}
	}

	// Read core-power feature info
	if rsp, err = sendMboxCmd(cpu, READ_PM_CONFIG, PM_FEATURE, 0); err != nil {
		return info, fmt.Errorf("failed to read SST CP info: %v", err)
	}

	info.CPSupported = isBitSet(rsp, 0)
	info.CPEnabled = isBitSet(rsp, 16)

	if info.CPSupported {
		if rsp, err = sendMboxCmd(cpu, CONFIG_CLOS, CLOS_PM_QOS_CONFIG, 0); err != nil {
			return info, fmt.Errorf("failed to read SST CP status: %v", err)
		}

		closEnabled := isBitSet(rsp, 1)
		if closEnabled != info.CPEnabled {
			log.Warn("SST firmware returned conflicting CP enabled status %v vs. %v", info.CPEnabled, closEnabled)
		}
		info.CPEnabled = closEnabled
		info.CPPriority = CPPriorityType(getBits(rsp, 2, 2))

		for i := uint32(0); i < NumClos; i++ {
			if rsp, err = sendMMIOCmd(cpu, (i<<2)+PM_CLOS_OFFSET); err != nil {
				return info, fmt.Errorf("failed to read SST CLOS #%d info: %v", i, err)
			}
			info.ClosInfo[i] = SstClosInfo{
				EPP:                  int(getBits(rsp, 0, 3)),
				ProportionalPriority: int(getBits(rsp, 4, 7)),
				MinFreq:              int(getBits(rsp, 8, 15)),
				MaxFreq:              int(getBits(rsp, 16, 23)),
				DesiredFreq:          int(getBits(rsp, 24, 31)),
			}
		}
	}

	return info, nil
}

func getCPUClosID(cpu ID) (int, error) {
	p, err := punitCPU(cpu)
	if err != nil {
		return -1, err
	}
	punitCore := uint32(p) >> 1

	rsp, err := sendMMIOCmd(cpu, (punitCore<<2)+PQR_ASSOC_OFFSET)
	if err != nil {
		return 0, fmt.Errorf("failed to read CLOS number of cpu %d: %v", cpu, err)
	}
	return int(getBits(rsp, 16, 17)), nil
}

func getCPUClosIDs(cpus []ID) ([]int, error) {
	ret := make([]int, len(cpus))
	for i, cpu := range cpus {
		id, err := getCPUClosID(cpu)
		if err != nil {
			return ret, err
		}
		ret[i] = id
	}
	return ret, nil
}

// IOCTL numbers
// Derived from kernel headers linux/isst_if.h
const (
	ISST_IF_GET_PHY_ID   = 0x000000c008fe01
	ISST_IF_IO_CMD       = 0x0000004008fe02
	ISST_IF_MBOX_COMMAND = 0x000000c008fe03
)

// Mbox command ids
// Originally from kernel sources linux/tools/power/x86/intel-speed-select/isst.h
const (
	// TDP (perf profile) related commands
	CONFIG_TDP                        = 0x7f // main command
	CONFIG_TDP_GET_LEVELS_INFO        = 0x00 // sub-command
	CONFIG_TDP_GET_TDP_CONTROL        = 0x01 // ...
	CONFIG_TDP_SET_TDP_CONTROL        = 0x02
	CONFIG_TDP_GET_TDP_INFO           = 0x03
	CONFIG_TDP_GET_PWR_INFO           = 0x04
	CONFIG_TDP_GET_TJMAX_INFO         = 0x05
	CONFIG_TDP_GET_CORE_MASK          = 0x06
	CONFIG_TDP_GET_TURBO_LIMIT_RATIOS = 0x07
	CONFIG_TDP_SET_LEVEL              = 0x08
	CONFIG_TDP_GET_UNCORE_P0_P1_INFO  = 0x09
	CONFIG_TDP_GET_P1_INFO            = 0x0a
	CONFIG_TDP_GET_MEM_FREQ           = 0x0b

	CONFIG_TDP_GET_FACT_HP_TURBO_LIMIT_NUMCORES = 0x10
	CONFIG_TDP_GET_FACT_HP_TURBO_LIMIT_RATIOS   = 0x11
	CONFIG_TDP_GET_FACT_LP_CLIPPING_RATIO       = 0x12

	CONFIG_TDP_PBF_GET_CORE_MASK_INFO = 0x20
	CONFIG_TDP_PBF_GET_P1HI_P1LO_INFO = 0x21
	CONFIG_TDP_PBF_GET_TJ_MAX_INFO    = 0x22
	CONFIG_TDP_PBF_GET_TDP_INFO       = 0x23

	// CLOS related commands
	CONFIG_CLOS        = 0xd0 // main command
	CLOS_PM_QOS_CONFIG = 0x02
	// The following are unusable
	//CLOS_PQR_ASSOC     = 0x00 // sub-commands
	//CLOS_PM_CLOS       = 0x01 // ..
	//CLOS_STATUS        = 0x03

	// PM commands
	READ_PM_CONFIG  = 0x94 // main command
	WRITE_PM_CONFIG = 0x95 // main command
	PM_FEATURE      = 0x03 // sub-command
)

// MMIO command definitions
// Originally from kernel sources linux/tools/power/x86/intel-speed-select/isst.h
const (
	PM_QOS_INFO_OFFSET   = 0x00
	PM_QOS_CONFIG_OFFSET = 0x04
	PM_CLOS_OFFSET       = 0x08
	PQR_ASSOC_OFFSET     = 0x20
)

// isstIfCPUMaps is used with ISST_IF_GET_PHY_ID ioctl to convert from Linux
// logical CPU numbers to PUNIT CPU numbers.
// Derived from kernel headers linux/isst_if.h
type isstIfCPUMaps struct {
	cmdCount uint32
	cpuMap   [maxIsstIfCPUMaps]isstIfCPUMap
}

const maxIsstIfCPUMaps = 1

// Derived from kernel headers linux/isst_if.h
type isstIfCPUMap struct {
	logicalCPU  uint32
	physicalCPU uint32
}

// isstIfMboxCmds is used with ISST_IF_MBOX_COMMAND ioctl to send mailbox
// commands to PUNIT.
// Derived from kernel headers linux/isst_if.h
type isstIfMboxCmds struct {
	cmdCount uint32
	mboxCmd  [maxIsstIfMboxCmds]isstIfMboxCmd
}

const maxIsstIfMboxCmds = 1

// Derived from kernel headers linux/isst_if.h
type isstIfMboxCmd struct {
	logicalCPU uint32
	parameter  uint32
	reqData    uint32
	respData   uint32
	command    uint16
	subCommand uint16
	reserved   uint32
}

// isstIfIoRegs is used with ISST_IF_IO_CMD ioctl to send commands to PUNIT.
// Derived from kernel headers linux/isst_if.h
type isstIfIoRegs struct {
	reqCount uint32
	ioReg    [maxIsstIfIoRegs]isstIfIoReg
}

const maxIsstIfIoRegs = 1

// Derived from kernel headers linux/isst_if.h
type isstIfIoReg struct {
	readWrite  uint32 // 0=R 1=W
	logicalCPU uint32
	reg        uint32
	value      uint32
}

var log = logger.NewLogger("sst")

// cpuMap holds the logical to punit cpu mapping table
var cpuMap = make(map[ID]ID)

func punitCPU(cpu ID) (ID, error) {
	if id, ok := cpuMap[cpu]; ok {
		return id, nil
	}

	id, err := getCPUMapping(cpu)
	if err == nil {
		cpuMap[cpu] = id
	}
	return id, err
}

// isstIoctl is a helper for executing ioctls on the linux isst_if device driver
func isstIoctl(ioctl uintptr, req uintptr) error {
	f, err := os.Open(isstDevPath)
	if err != nil {
		return fmt.Errorf("failed to open isst device %q: %v", isstDevPath, err)
	}
	defer f.Close()

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(f.Fd()), ioctl, req); errno != 0 {
		return errno
	}

	return nil
}

// getCPUMapping gets mapping of Linux logical CPU numbers to (package-specific)
// PUNIT CPU number for one cpu
func getCPUMapping(cpu ID) (ID, error) {
	req := isstIfCPUMaps{
		cmdCount: 1,
		cpuMap: [maxIsstIfCPUMaps]isstIfCPUMap{
			{logicalCPU: uint32(cpu)},
		},
	}

	if err := isstIoctl(ISST_IF_GET_PHY_ID, uintptr(unsafe.Pointer(&req))); err != nil {
		return -1, fmt.Errorf("failed to get CPU mapping for cpu %d: %v", cpu, err)
	}

	return ID(req.cpuMap[0].physicalCPU), nil
}

// sendMboxCmd sends one mailbox command to PUNIT
func sendMboxCmd(cpu ID, cmd uint16, subCmd uint16, reqData uint32) (uint32, error) {
	req := isstIfMboxCmds{
		cmdCount: 1,
		mboxCmd: [maxIsstIfMboxCmds]isstIfMboxCmd{
			{
				logicalCPU: uint32(cpu),
				command:    cmd,
				subCommand: subCmd,
				reqData:    reqData,
			},
		},
	}

	log.Debug("MBOX SEND cpu: %d cmd: %#02x sub: %#02x data: %#x", cpu, cmd, subCmd, reqData)
	if err := isstIoctl(ISST_IF_MBOX_COMMAND, uintptr(unsafe.Pointer(&req))); err != nil {
		return 0, fmt.Errorf("Mbox command failed with %v", err)
	}
	log.Debug("MBOX RECV data: %#x", req.mboxCmd[0].respData)

	return req.mboxCmd[0].respData, nil
}

func sendMMIOCmd(cpu ID, reg uint32) (uint32, error) {
	req := isstIfIoRegs{
		reqCount: 1,
		ioReg: [maxIsstIfIoRegs]isstIfIoReg{
			{
				logicalCPU: uint32(cpu),
				reg:        reg,
			},
		},
	}
	log.Debug("MMIO SEND cpu: %d reg: %#x", cpu, reg)
	if err := isstIoctl(ISST_IF_IO_CMD, uintptr(unsafe.Pointer(&req))); err != nil {
		return 0, fmt.Errorf("MMIO command failed with %v", err)
	}
	log.Debug("MMIO RECV data: %#x", req.ioReg[0].value)

	return req.ioReg[0].value, nil
}

func getBits(val, i, j uint32) uint32 {
	lsb := i
	msb := j
	if i > j {
		lsb = j
		msb = i
	}
	return (val >> lsb) & ((1 << (msb - lsb + 1)) - 1)
}

func isBitSet(val, n uint32) bool {
	return val&(1<<n) != 0
}
