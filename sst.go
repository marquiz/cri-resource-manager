package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"

	"github.com/intel/cri-resource-manager/pkg/sysfs"
)

func getLogicalCpus() ([]int, error) {

	d, err := ioutil.ReadDir("/sys/bus/cpu/devices")
	cpus := make([]int, len(d))
	if err != nil {
		return cpus, fmt.Errorf("failed to read CPUs: %v", err)
	}

	for i, f := range d {
		if n, err := strconv.Atoi(f.Name()[3:]); err != nil {
			return cpus, fmt.Errorf("failed to parse cpu number from %q: %v", f.Name(), err)
		} else {
			cpus[i] = n
		}
	}

	sort.Slice(cpus, func(i, j int) bool { return cpus[i] < cpus[j] })
	return cpus, nil
}

func main() {
	var err error
	cpuId := 0
	if len(os.Args) > 1 {
		if cpuId, err = strconv.Atoi(os.Args[1]); err != nil {
			log.Fatalf("invalid cpuid %q: %v", os.Args[1], err)
		}
	}

	/*	cpus, err := getLogicalCpus()
		if err != nil {
			log.Fatal(err)
		}
		cpuMap, err := sst.GetCpuMappings(cpus)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Logical -> physical CPU mapping")
		for i, n := range cpus {
			fmt.Printf("  %3d: %3d |", n, cpuMap[n])
			if (i+1)%10 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()*/

	sys, err := sysfs.DiscoverSystem()
	if err != nil {
		log.Fatal(err)
	}

	cpu := sys.CPU(sysfs.ID(cpuId))
	pkgId := cpu.PackageID()
	pkg := sys.Package(pkgId)

	info := pkg.SstInfo()

	fmt.Printf("Status for CPU %d / PACKAGE: %d:\n", cpuId, pkgId)

	pp := "disabled"
	if info.PPSupported {
		pp = "enabled"
	}
	fmt.Printf(" SST-PP: %s\n", pp)
	fmt.Printf("    current level: %d\n    max level: %d\n    locked: %v\n    version: %#x\n",
		info.PPCurrentLevel, info.PPMaxLevel, info.PPLocked, info.PPVersion)

	cp := "not supported"
	switch {
	case info.CPEnabled:
		cp = "enabled"
	case info.CPSupported:
		cp = "disabled"
	}
	fmt.Printf("  SST-CP: %s\n", cp)
	if info.CPEnabled {
		closPrio := map[sysfs.CPPriorityType]string{sysfs.Ordered: "ordered", sysfs.Proportional: "proportional"}[info.CPPriority]
		fmt.Printf("    priority: %s\n", closPrio)
	}

	tf := "not supported"
	switch {
	case info.TFEnabled:
		tf = "enabled"
	case info.TFSupported:
		tf = "disabled"
	}
    fmt.Printf("  SST-TF: %s\n", tf)

	bf := "not supported"
	switch {
	case info.BFEnabled:
		bf = "enabled"
	case info.BFSupported:
		bf = "disabled"
	}
    fmt.Printf("  SST-BF: %s\n", bf)
	fmt.Printf("    priority cores: %v\n", info.BFCores)

	if info.CPEnabled {
		closNum := cpu.SstClos()
		fmt.Printf("  CLOS ID: %d\n", closNum)

		ci := info.ClosInfo[closNum]
		fmt.Printf("    epp: %d\n    prio: %d\n    min: %d\n    max: %d\n    desired: %d\n",
			ci.EPP, ci.ProportionalPriority, ci.MinFreq, ci.MaxFreq, ci.DesiredFreq)
	}
}
