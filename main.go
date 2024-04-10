package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/shirou/gopsutil/process"
)

type Process struct {
	Pid    int32
	Name   string
	Memory *process.MemoryInfoStat
}

func main() {

	if len(os.Args) == 1 {
		fmt.Fprint(os.Stderr, "Usage: mem <comma_separated_process_names>\n")
		os.Exit(1)
	}

	processNames := strings.Split(os.Args[1], ",")

	parentsMap := make(map[string]struct{})
	for _, proc := range processNames {
		parentsMap[proc] = struct{}{}
	}

	processes, err := process.ProcessesWithContext(context.Background())
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	results := make(map[int32][]Result)

	fmt.Printf("A total of %d processes running.\n", len(processes))

	resultChan := make(chan Result, len(processes))
	count := 0
	for _, proc := range processes {
		if proc != nil {
			count++
			go getProcessInfo(resultChan, parentsMap, proc)

		}
	}

	for range count {
		r := <-resultChan
		if r.err == nil {
			results[r.parentPID] = append(results[r.parentPID], r)
		}
	}
	var data [][]string

	for parentPID, results := range results {
		var rssMem uint64
		for _, result := range results {
			if result.process.Memory != nil {
				rssMem = result.process.Memory.RSS / (1024 * 1024)
			}
			data = append(data, []string{
				result.process.Name,
				fmt.Sprintf("%dMB", rssMem),
				fmt.Sprintf("%d", result.process.Pid),
				result.parentName,
				fmt.Sprintf("%d", parentPID),
			})

		}
	}
	if len(data) == 0 {
		os.Exit(0)
	}

	re := lipgloss.NewRenderer(os.Stdout)
	baseStyle := re.NewStyle().Padding(0, 1)
	headerStyle := baseStyle.Copy().Foreground(lipgloss.Color("252")).Bold(true)
	headers := []string{"Process", "RSS Mem", "PID", "Parent", "Parent PID"}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(re.NewStyle().Foreground(lipgloss.Color("238"))).
		Headers(headers...).
		Rows(data...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}

			even := row%2 == 0

			if even {
				return baseStyle.Copy().Foreground(lipgloss.Color("245"))
			}
			return baseStyle.Copy().Foreground(lipgloss.Color("252"))
		})
	fmt.Println(t)
}

type Result struct {
	parentPID  int32
	parentName string
	process    Process
	err        error
}

func getProcessInfo(resultChan chan<- Result, parentsMap map[string]struct{}, proc *process.Process) {
	name, err := proc.Name()
	if err != nil {
		resultChan <- Result{err: err}
		return
	}
	_, isParent := parentsMap[name]
	if isParent {
		resultChan <- Result{err: fmt.Errorf("Parent process")}
		return
	}

	parent, err := proc.Parent()
	if err != nil {
		resultChan <- Result{err: err}
		return
	}
	pname, err := parent.Name()
	if err != nil {
		resultChan <- Result{parentPID: parent.Pid, err: err}
		return
	}

	_, ok := parentsMap[pname]

	if ok {
		memInfo, err := proc.MemoryInfo()
		if err != nil {
			resultChan <- Result{parentPID: parent.Pid, parentName: pname, err: err}
			return
		}
		p := Process{
			proc.Pid,
			name,
			memInfo,
		}
		resultChan <- Result{parentPID: parent.Pid, parentName: pname, process: p}
		return
	}
	resultChan <- Result{err: fmt.Errorf("Uninteresting process")}
}
