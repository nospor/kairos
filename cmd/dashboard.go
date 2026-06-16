package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nospor/kairos/db"
	"github.com/nospor/kairos/model"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"tui"},
	Short:   "Start the interactive terminal user interface",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

type tabId int

const (
	tabDashboard tabId = iota
	tabProjects
	tabHistory
	tabReport
)

type reportTimeFilterType int

const (
	reportTimeAll reportTimeFilterType = iota
	reportTimeToday
	reportTimeWeek
	reportTimeMonth
)

type reportLine struct {
	text     string
	isHeader bool
	isTotal  bool
}

type activePane int

const (
	paneProjects activePane = iota
	paneTasks
)

type modalType int

const (
	modalNone modalType = iota
	modalCreateProject
	modalRenameProject
	modalCreateTask
	modalRenameTask
	modalStartSelectProj
	modalStartSelectTask
	modalStartCreateTaskName
	modalConfirmDeleteProj
	modalConfirmDeleteTask
	modalConfirmDeleteEntry
	modalUpdateEntryTime
	modalReportSelectProj
	modalReportSelectTime
	modalReportSelectGroup
)

// tickMsg is sent periodically to update time and poll database
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// mainModel holds the TUI application state
type mainModel struct {
	activeTab  tabId
	activeTask *model.ActiveInfo
	projects   []model.ProjectInfo
	history    []model.HistoryEntry

	// Cursors
	projectCursor int
	taskCursor    int
	historyCursor int
	reportCursor  int

	// Pagination/Scroll offsets
	historyOffset int
	historyLimit  int

	// Report state
	reportTimeFilter  reportTimeFilterType
	reportGroupFilter string
	reportProjFilter  string
	reportLines       []reportLine
	reportOffset      int
	reportLimit       int

	// Focus pane on Tab 2
	activePane activePane

	// Modal prompt state
	modal         modalType
	input         textinput.Model
	startTaskName string // Temp wizard state

	// Project/Task Select values for Wizard
	projectSelectCursor int
	taskSelectCursor    int
	projectSelectNames  []string
	taskSelectNames     []string
	selectedProj        string

	// Notifications
	errorMsg   string
	successMsg string
	msgExpiry  time.Time

	// Viewport size
	width  int
	height int
}

func initialModel() mainModel {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()

	return mainModel{
		activeTab:    tabDashboard,
		activePane:   paneProjects,
		modal:        modalNone,
		input:        ti,
		historyLimit: 15,
		reportLimit:  15,
	}
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		func() tea.Msg {
			return "refresh"
		},
	)
}

func (m *mainModel) refresh() {
	// 1. Fetch active task
	if active, err := store.GetActiveTask(); err == nil {
		m.activeTask = active
	} else {
		m.activeTask = nil
	}

	// 2. Fetch projects & tasks
	if projs, err := store.ListProjects(); err == nil {
		m.projects = projs
	}

	// 3. Fetch history
	if hist, err := store.GetHistory(100); err == nil {
		m.history = hist
	}

	// 4. Generate report lines
	var rFilter db.ReportFilter
	rFilter.ProjectName = m.reportProjFilter
	rFilter.GroupBy = m.reportGroupFilter

	now := time.Now()
	switch m.reportTimeFilter {
	case reportTimeToday:
		bod := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		eod := bod.Add(24 * time.Hour)
		rFilter.From = &bod
		rFilter.To = &eod
	case reportTimeWeek:
		weekday := now.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -int(weekday-time.Monday))
		bow := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
		eow := bow.AddDate(0, 0, 7)
		rFilter.From = &bow
		rFilter.To = &eow
	case reportTimeMonth:
		bom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		eom := bom.AddDate(0, 1, 0)
		rFilter.From = &bom
		rFilter.To = &eom
	}

	if rows, err := store.GetReport(rFilter); err == nil {
		widthVal := max(50, m.width-6)
		innerWidth := widthVal - 8
		m.reportLines = m.generateReportLines(rows, innerWidth)
	} else {
		m.reportLines = nil
	}

	// Adjust cursors in case lists shrunk
	if m.projectCursor >= len(m.projects) {
		m.projectCursor = max(0, len(m.projects)-1)
	}
	if len(m.projects) > 0 {
		tasks := m.projects[m.projectCursor].Tasks
		if m.taskCursor >= len(tasks) {
			m.taskCursor = max(0, len(tasks)-1)
		}
	} else {
		m.taskCursor = 0
	}
	if m.historyCursor >= len(m.history) {
		m.historyCursor = max(0, len(m.history)-1)
	}

	contentLinesCount := len(m.reportLines) - 2
	if contentLinesCount < 0 {
		contentLinesCount = 0
	}
	if m.reportCursor >= contentLinesCount {
		m.reportCursor = max(0, contentLinesCount-1)
	}
}

func (m *mainModel) setSuccess(msg string) {
	m.successMsg = msg
	m.errorMsg = ""
	m.msgExpiry = time.Now().Add(4 * time.Second)
}

func (m *mainModel) setError(msg string) {
	m.errorMsg = msg
	m.successMsg = ""
	m.msgExpiry = time.Now().Add(4 * time.Second)
}

func (m *mainModel) clearMsgs() {
	if time.Now().After(m.msgExpiry) {
		m.errorMsg = ""
		m.successMsg = ""
	}
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust history and report viewport limits based on height
		m.historyLimit = max(3, m.height-19)
		m.reportLimit = max(3, m.height-19)
		m.refresh()
		return m, nil

	case string:
		if msg == "refresh" {
			m.refresh()
		}

	case tickMsg:
		m.refresh()
		m.clearMsgs()
		return m, tickCmd()
	}

	// If modal is active, process inputs for modal
	if m.modal != modalNone {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if m.modal == modalReportSelectProj {
				switch msg.String() {
				case "esc":
					m.modal = modalNone
					return m, nil
				case "up", "k":
					if m.projectSelectCursor > 0 {
						m.projectSelectCursor--
					}
					return m, nil
				case "down", "j":
					if m.projectSelectCursor < len(m.projectSelectNames)-1 {
						m.projectSelectCursor++
					}
					return m, nil
				case "enter":
					selected := m.projectSelectNames[m.projectSelectCursor]
					if selected == "[ All Projects ]" {
						m.reportProjFilter = ""
					} else {
						m.reportProjFilter = selected
					}
					m.modal = modalNone
					m.reportOffset = 0
					m.reportCursor = 0
					m.refresh()
					return m, nil
				}
				return m, nil
			}

			if m.modal == modalReportSelectTime {
				switch msg.String() {
				case "esc":
					m.modal = modalNone
					return m, nil
				case "up", "k":
					if m.projectSelectCursor > 0 {
						m.projectSelectCursor--
					}
					return m, nil
				case "down", "j":
					if m.projectSelectCursor < len(m.projectSelectNames)-1 {
						m.projectSelectCursor++
					}
					return m, nil
				case "enter":
					m.reportTimeFilter = reportTimeFilterType(m.projectSelectCursor)
					m.modal = modalNone
					m.reportOffset = 0
					m.reportCursor = 0
					m.refresh()
					return m, nil
				}
				return m, nil
			}

			if m.modal == modalReportSelectGroup {
				switch msg.String() {
				case "esc":
					m.modal = modalNone
					return m, nil
				case "up", "k":
					if m.projectSelectCursor > 0 {
						m.projectSelectCursor--
					}
					return m, nil
				case "down", "j":
					if m.projectSelectCursor < len(m.projectSelectNames)-1 {
						m.projectSelectCursor++
					}
					return m, nil
				case "enter":
					switch m.projectSelectCursor {
					case 0:
						m.reportGroupFilter = ""
					case 1:
						m.reportGroupFilter = "day"
					case 2:
						m.reportGroupFilter = "week"
					case 3:
						m.reportGroupFilter = "month"
					case 4:
						m.reportGroupFilter = "year"
					}
					m.modal = modalNone
					m.reportOffset = 0
					m.reportCursor = 0
					m.refresh()
					return m, nil
				}
				return m, nil
			}

			// If we are in selection modals, handle selection keys
			if m.modal == modalStartSelectProj {
				switch msg.String() {
				case "esc":
					m.modal = modalNone
					return m, nil
				case "up", "k":
					if m.projectSelectCursor > 0 {
						m.projectSelectCursor--
					}
					return m, nil
				case "down", "j":
					if m.projectSelectCursor < len(m.projectSelectNames)-1 {
						m.projectSelectCursor++
					}
					return m, nil
				case "enter":
					m.selectedProj = m.projectSelectNames[m.projectSelectCursor]
					// Populate task list for this project
					m.taskSelectNames = []string{}
					var foundProj *model.ProjectInfo
					for _, p := range m.projects {
						if p.ProjectName == m.selectedProj {
							foundProj = &p
							break
						}
					}
					if foundProj != nil {
						for _, t := range foundProj.Tasks {
							m.taskSelectNames = append(m.taskSelectNames, t.TaskName)
						}
					}
					m.taskSelectNames = append(m.taskSelectNames, "[ + Create & Start New Task... ]")
					m.taskSelectCursor = 0
					m.modal = modalStartSelectTask
					return m, nil
				}
				return m, nil
			}

			if m.modal == modalStartSelectTask {
				switch msg.String() {
				case "esc":
					m.modal = modalStartSelectProj
					return m, nil
				case "up", "k":
					if m.taskSelectCursor > 0 {
						m.taskSelectCursor--
					}
					return m, nil
				case "down", "j":
					if m.taskSelectCursor < len(m.taskSelectNames)-1 {
						m.taskSelectCursor++
					}
					return m, nil
				case "enter":
					selectedTask := m.taskSelectNames[m.taskSelectCursor]
					if selectedTask == "[ + Create & Start New Task... ]" {
						m.modal = modalStartCreateTaskName
						m.input.SetValue("")
						m.input.Prompt = "New Task Name: "
						m.input.Focus()
						return m, nil
					} else {
						if err := store.StartTask(selectedTask, m.selectedProj); err != nil {
							m.setError(err.Error())
						} else {
							m.setSuccess(fmt.Sprintf("Started tracking %q.", selectedTask))
							startDaemon(0)
						}
						m.modal = modalNone
						m.refresh()
						return m, nil
					}
				}
				return m, nil
			}

			switch msg.String() {
			case "esc":
				m.modal = modalNone
				m.input.Blur()
				return m, nil

			case "enter":
				// Confirm deletion
				if m.modal == modalConfirmDeleteProj {
					projName := m.projects[m.projectCursor].ProjectName
					if err := store.DeleteProject(projName); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Project %q deleted.", projName))
						m.projectCursor = 0
						m.taskCursor = 0
					}
					m.modal = modalNone
					m.refresh()
					return m, nil
				}
				if m.modal == modalConfirmDeleteTask {
					taskName := m.projects[m.projectCursor].Tasks[m.taskCursor].TaskName
					if err := store.DeleteTask(taskName); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Task %q deleted.", taskName))
						m.taskCursor = 0
					}
					m.modal = modalNone
					m.refresh()
					return m, nil
				}
				if m.modal == modalConfirmDeleteEntry {
					entryID := m.history[m.historyCursor].ID
					if err := store.DeleteTimeEntry(entryID); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess("Time entry deleted.")
					}
					m.modal = modalNone
					m.refresh()
					return m, nil
				}

				// Form submits
				val := strings.TrimSpace(m.input.Value())
				switch m.modal {
				case modalCreateProject:
					if val == "" {
						m.setError("Project name cannot be empty.")
					} else if err := store.CreateProject(val); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Project %q created.", val))
					}
				case modalRenameProject:
					oldName := m.projects[m.projectCursor].ProjectName
					if val == "" {
						m.setError("Project name cannot be empty.")
					} else if err := store.RenameProject(oldName, val); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Project %q renamed to %q.", oldName, val))
					}
				case modalCreateTask:
					projName := m.projects[m.projectCursor].ProjectName
					if val == "" {
						m.setError("Task name cannot be empty.")
					} else if err := store.CreateTask(val, projName); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Task %q created in %q.", val, projName))
					}
				case modalRenameTask:
					projName := m.projects[m.projectCursor].ProjectName
					oldName := m.projects[m.projectCursor].Tasks[m.taskCursor].TaskName
					if val == "" {
						m.setError("Task name cannot be empty.")
					} else if err := store.RenameTask(oldName, val, projName); err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Task %q renamed to %q.", oldName, val))
					}
				case modalStartCreateTaskName:
					if val == "" {
						m.setError("Task name cannot be empty.")
					} else {
						// Ensure task is created
						_ = store.CreateTask(val, m.selectedProj)
						if err := store.StartTask(val, m.selectedProj); err != nil {
							m.setError(err.Error())
						} else {
							m.setSuccess(fmt.Sprintf("Started tracking %q.", val))
							startDaemon(0)
						}
					}
				case modalUpdateEntryTime:
					if val == "" {
						m.setError("Duration cannot be empty.")
					} else {
						duration, err := time.ParseDuration(val)
						if err != nil {
							m.setError(fmt.Sprintf("Invalid duration: %v", err))
						} else if duration <= 0 {
							m.setError("Duration must be positive.")
						} else {
							h := m.history[m.historyCursor]
							entry, err := store.GetTimeEntry(h.ID)
							if err != nil {
								m.setError(err.Error())
							} else {
								startAt := entry.StartAt
								var stopAt *time.Time
								if entry.StopAt != nil {
									t := entry.StartAt.Add(duration)
									stopAt = &t
								} else {
									startAt = time.Now().Add(-duration)
								}

								if err := store.UpdateTimeEntry(h.ID, startAt, stopAt); err != nil {
									m.setError(err.Error())
								} else {
									m.setSuccess("Time entry updated successfully.")
								}
							}
						}
					}
				}
				m.modal = modalNone
				m.input.Blur()
				m.refresh()
				return m, nil

			// For confirmation dialogs, support y/n keys directly
			default:
				if m.modal == modalConfirmDeleteProj || m.modal == modalConfirmDeleteTask || m.modal == modalConfirmDeleteEntry {
					k := msg.String()
					if strings.ToLower(k) == "y" {
						// perform delete
						if m.modal == modalConfirmDeleteProj {
							projName := m.projects[m.projectCursor].ProjectName
							if err := store.DeleteProject(projName); err != nil {
								m.setError(err.Error())
							} else {
								m.setSuccess(fmt.Sprintf("Project %q deleted.", projName))
								m.projectCursor = 0
								m.taskCursor = 0
							}
						} else if m.modal == modalConfirmDeleteTask {
							taskName := m.projects[m.projectCursor].Tasks[m.taskCursor].TaskName
							if err := store.DeleteTask(taskName); err != nil {
								m.setError(err.Error())
							} else {
								m.setSuccess(fmt.Sprintf("Task %q deleted.", taskName))
								m.taskCursor = 0
							}
						} else if m.modal == modalConfirmDeleteEntry {
							entryID := m.history[m.historyCursor].ID
							if err := store.DeleteTimeEntry(entryID); err != nil {
								m.setError(err.Error())
							} else {
								m.setSuccess("Time entry deleted.")
							}
							m.historyCursor = max(0, m.historyCursor-1)
						}
						m.modal = modalNone
						m.refresh()
						return m, nil
					} else if strings.ToLower(k) == "n" {
						m.modal = modalNone
						return m, nil
					}
				}
			}
		}

		// Update text input
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// Normal view keys
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "1":
			m.activeTab = tabDashboard
		case "2":
			m.activeTab = tabProjects
		case "3":
			m.activeTab = tabHistory
		case "4":
			m.activeTab = tabReport

		case "tab":
			m.activeTab = (m.activeTab + 1) % 4

		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + 4) % 4

		case "up", "k":
			switch m.activeTab {
			case tabProjects:
				if m.activePane == paneProjects {
					if m.projectCursor > 0 {
						m.projectCursor--
						m.taskCursor = 0
					}
				} else {
					if m.taskCursor > 0 {
						m.taskCursor--
					}
				}
			case tabHistory:
				if m.historyCursor > 0 {
					m.historyCursor--
					if m.historyCursor < m.historyOffset {
						m.historyOffset = m.historyCursor
					}
				}
			case tabReport:
				if m.reportCursor > 0 {
					m.reportCursor--
					// Skip empty spacer lines
					if m.reportCursor > 0 && m.reportLines[2+m.reportCursor].text == "" {
						m.reportCursor--
					}
					if m.reportCursor < m.reportOffset {
						m.reportOffset = m.reportCursor
					}
				}
			}

		case "down", "j":
			switch m.activeTab {
			case tabProjects:
				if m.activePane == paneProjects {
					if m.projectCursor < len(m.projects)-1 {
						m.projectCursor++
						m.taskCursor = 0
					}
				} else {
					if len(m.projects) > 0 {
						tasks := m.projects[m.projectCursor].Tasks
						if m.taskCursor < len(tasks)-1 {
							m.taskCursor++
						}
					}
				}
			case tabHistory:
				if m.historyCursor < len(m.history)-1 {
					m.historyCursor++
					if m.historyCursor >= m.historyOffset+m.historyLimit {
						m.historyOffset = m.historyCursor - m.historyLimit + 1
					}
				}
			case tabReport:
				contentLines := m.reportLines[2:]
				if m.reportCursor < len(contentLines)-1 {
					m.reportCursor++
					// Skip empty spacer lines
					if m.reportCursor < len(contentLines) && contentLines[m.reportCursor].text == "" {
						if m.reportCursor < len(contentLines)-1 {
							m.reportCursor++
						} else {
							m.reportCursor--
						}
					}
					limit := m.reportLimit - 2
					if limit < 3 {
						limit = 3
					}
					if m.reportCursor >= m.reportOffset+limit {
						m.reportOffset = m.reportCursor - limit + 1
					}
				}
			}

		case "left", "h":
			if m.activeTab == tabProjects {
				m.activePane = paneProjects
			}

		case "right", "l":
			if m.activeTab == tabProjects {
				m.activePane = paneTasks
			}

		// Tab 1 (Dashboard) Action Keys
		case "s":
			if m.activeTab == tabDashboard {
				if m.activeTask != nil {
					name, dur, err := store.StopActive()
					if err != nil {
						m.setError(err.Error())
					} else {
						m.setSuccess(fmt.Sprintf("Stopped tracking %q (Duration: %s).", name, formatDuration(dur)))
					}
					m.refresh()
				} else {
					m.setError("No active task is running.")
				}
			}

		case "n":
			if m.activeTab == tabDashboard {
				if m.activeTask != nil {
					m.setError("A task is already running. Stop it first.")
				} else {
					m.refresh()
					m.projectSelectNames = []string{}
					for _, proj := range m.projects {
						m.projectSelectNames = append(m.projectSelectNames, proj.ProjectName)
					}
					if len(m.projectSelectNames) == 0 {
						m.projectSelectNames = append(m.projectSelectNames, "General")
					}
					m.projectSelectCursor = 0
					m.modal = modalStartSelectProj
				}
			}

		// Tab 4 (Report) Action Keys
		case "p":
			if m.activeTab == tabReport {
				m.projectSelectNames = []string{"[ All Projects ]"}
				for _, proj := range m.projects {
					m.projectSelectNames = append(m.projectSelectNames, proj.ProjectName)
				}
				m.projectSelectCursor = 0
				m.modal = modalReportSelectProj
			}

		case "t":
			if m.activeTab == tabReport {
				m.projectSelectNames = []string{"[ All Time ]", "[ Today ]", "[ This Week ]", "[ This Month ]"}
				m.projectSelectCursor = int(m.reportTimeFilter)
				m.modal = modalReportSelectTime
			}

		case "g":
			if m.activeTab == tabReport {
				m.projectSelectNames = []string{"[ No Grouping ]", "[ Group by Day ]", "[ Group by Week ]", "[ Group by Month ]", "[ Group by Year ]"}
				switch m.reportGroupFilter {
				case "":
					m.projectSelectCursor = 0
				case "day":
					m.projectSelectCursor = 1
				case "week":
					m.projectSelectCursor = 2
				case "month":
					m.projectSelectCursor = 3
				case "year":
					m.projectSelectCursor = 4
				}
				m.modal = modalReportSelectGroup
			}

		case "r":
			if m.activeTab == tabReport {
				m.reportProjFilter = ""
				m.reportTimeFilter = reportTimeAll
				m.reportGroupFilter = ""
				m.reportOffset = 0
				m.reportCursor = 0
				m.refresh()
			}

		// Tab 2 & 3 Action Keys
		case "c":
			if m.activeTab == tabProjects {
				m.input.SetValue("")
				m.input.Focus()
				if m.activePane == paneProjects {
					m.modal = modalCreateProject
					m.input.Prompt = "Project Name: "
				} else {
					if len(m.projects) == 0 {
						m.setError("Create a project first.")
						m.input.Blur()
					} else {
						m.modal = modalCreateTask
						m.input.Prompt = "Task Name: "
					}
				}
			}

		case "e":
			if m.activeTab == tabProjects {
				m.input.Focus()
				if m.activePane == paneProjects {
					if len(m.projects) > 0 {
						name := m.projects[m.projectCursor].ProjectName
						m.modal = modalRenameProject
						m.input.SetValue(name)
						m.input.Prompt = "New Project Name: "
					} else {
						m.input.Blur()
					}
				} else {
					if len(m.projects) > 0 && len(m.projects[m.projectCursor].Tasks) > 0 {
						name := m.projects[m.projectCursor].Tasks[m.taskCursor].TaskName
						m.modal = modalRenameTask
						m.input.SetValue(name)
						m.input.Prompt = "New Task Name: "
					} else {
						m.input.Blur()
					}
				}
			}

		case "d", "backspace", "delete":
			if m.activeTab == tabProjects {
				if m.activePane == paneProjects {
					if len(m.projects) > 0 {
						name := m.projects[m.projectCursor].ProjectName
						if name == "General" {
							m.setError("Cannot delete the default 'General' project.")
						} else {
							m.modal = modalConfirmDeleteProj
						}
					}
				} else {
					if len(m.projects) > 0 && len(m.projects[m.projectCursor].Tasks) > 0 {
						m.modal = modalConfirmDeleteTask
					}
				}
			} else if m.activeTab == tabHistory {
				if len(m.history) > 0 {
					m.modal = modalConfirmDeleteEntry
				}
			}

		case "u":
			if m.activeTab == tabHistory {
				if len(m.history) > 0 {
					m.input.SetValue(m.history[m.historyCursor].Duration.Round(time.Second).String())
					m.input.Focus()
					m.modal = modalUpdateEntryTime
					m.input.Prompt = "Duration (e.g. 1h30m, 45m): "
				}
			}

		case "enter":
			if m.activeTab == tabProjects {
				if len(m.projects) > 0 && len(m.projects[m.projectCursor].Tasks) > 0 {
					task := m.projects[m.projectCursor].Tasks[m.taskCursor]
					if m.activeTask != nil {
						m.setError("A task is already running. Stop it first.")
					} else {
						if err := store.StartTask(task.TaskName, task.ProjectName); err != nil {
							m.setError(err.Error())
						} else {
							m.setSuccess(fmt.Sprintf("Started tracking %q.", task.TaskName))
							startDaemon(0)
						}
						m.refresh()
					}
				}
			} else if m.activeTab == tabHistory {
				if len(m.history) > 0 {
					entry := m.history[m.historyCursor]
					if m.activeTask != nil {
						m.setError("A task is already running. Stop it first.")
					} else {
						if err := store.StartTask(entry.TaskName, entry.ProjectName); err != nil {
							m.setError(err.Error())
						} else {
							m.setSuccess(fmt.Sprintf("Restarted tracking %q.", entry.TaskName))
							startDaemon(0)
						}
						m.refresh()
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// Lipgloss theme and styles
var (
	colorPurple   = lipgloss.Color("#7D56F4")
	colorCyan     = lipgloss.Color("#00F0FF")
	colorPink     = lipgloss.Color("#FF007F")
	colorGreen    = lipgloss.Color("#00E676")
	colorRed      = lipgloss.Color("#FF2E93")
	colorGray     = lipgloss.Color("#888888")
	colorDarkGray = lipgloss.Color("#333333")
	colorText     = lipgloss.Color("#EEEEEE")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPurple).
			Padding(0, 2).
			MarginRight(2)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorCyan).
			Padding(0, 1).
			MarginRight(2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorGray).
				Padding(0, 1).
				MarginRight(2)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 2)

	styleBoxActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(1, 2)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorPurple)

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			Background(lipgloss.Color("#2A2A35"))

	styleNormal = lipgloss.NewStyle().
			Foreground(colorText)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorGray)

	styleSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen)

	styleWarning = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorRed)

	styleHelpKey = lipgloss.NewStyle().
			Foreground(colorPurple)

	styleHelpDesc = lipgloss.NewStyle().
			Foreground(colorGray)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorPink).
			Padding(1, 3).
			Width(50)
)

func (m mainModel) View() string {
	var s strings.Builder

	// 1. Header
	s.WriteString(styleTitle.Render(" KAIROS TIME TRACKER ") + " ")
	s.WriteString(lipgloss.NewStyle().Foreground(colorGray).Italic(true).Render("Manage your time in style"))
	s.WriteString("\n\n")

	// 2. Tabs
	var tabs []string
	tabNames := []string{"1. DASHBOARD", "2. PROJECTS & TASKS", "3. RECENT HISTORY", "4. REPORT"}
	for i, name := range tabNames {
		if m.activeTab == tabId(i) {
			tabs = append(tabs, styleTabActive.Render(name))
		} else {
			tabs = append(tabs, styleTabInactive.Render(name))
		}
	}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	s.WriteString("\n\n")

	// 3. Main body based on tab / modal
	var body string
	if m.modal != modalNone {
		body = m.drawModal()
	} else {
		switch m.activeTab {
		case tabDashboard:
			body = m.drawDashboard()
		case tabProjects:
			body = m.drawProjects()
		case tabHistory:
			body = m.drawHistory()
		case tabReport:
			body = m.drawReport()
		}
	}

	s.WriteString(body)
	s.WriteString("\n")

	// 4. Toast notifications
	if m.errorMsg != "" {
		s.WriteString(styleWarning.Render("❌  "+m.errorMsg) + "\n")
	} else if m.successMsg != "" {
		s.WriteString(styleSuccess.Render("✔️  "+m.successMsg) + "\n")
	}

	// 5. Help Footer
	s.WriteString(m.drawHelp())
	s.WriteString("\n")

	return s.String()
}

func (m mainModel) drawDashboard() string {
	var body strings.Builder

	if m.activeTask != nil {
		duration := time.Since(m.activeTask.StartedAt)
		durStr := formatDuration(duration)

		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Render("⬤  ACTIVE TASK RUNNING") + "\n\n")
		body.WriteString("Task:    " + lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(m.activeTask.TaskName) + "\n")
		body.WriteString("Project: " + lipgloss.NewStyle().Foreground(colorPurple).Render(m.activeTask.ProjectName) + "\n")
		body.WriteString("Started: " + m.activeTask.StartedAt.Format("15:04:05 (MST)") + "\n\n")

		// Draw clock box
		clockBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGreen).
			Padding(1, 4).
			Align(lipgloss.Center).
			Render(
				lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Render(durStr) +
					"\n" +
					lipgloss.NewStyle().Faint(true).Foreground(colorText).Render("elapsed time"),
			)
		body.WriteString(clockBox)
	} else {
		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorGray).Render("⬤  STATUS: IDLE") + "\n\n")
		body.WriteString("No task is currently being tracked.\n\n")
		body.WriteString(styleMuted.Render("Press 'n' to start tracking a new task,") + "\n")
		body.WriteString(styleMuted.Render("or switch to 'PROJECTS & TASKS' (2) to select an existing task.") + "\n")
	}

	heightVal := max(8, m.height-10)
	return styleBoxActive.Height(heightVal).Width(max(40, m.width-6)).Render(body.String())
}

func (m mainModel) drawProjects() string {
	leftWidth := max(20, m.width/3)
	rightWidth := max(30, m.width-leftWidth-12)
	heightVal := max(8, m.height-10)

	leftInnerWidth := leftWidth - 6
	rightInnerWidth := rightWidth - 6

	// Projects list (Left)
	var leftSb strings.Builder
	leftSb.WriteString(styleHeader.Width(leftInnerWidth).Render("PROJECTS") + "\n")
	if len(m.projects) == 0 {
		leftSb.WriteString(styleMuted.Render("No projects."))
	} else {
		for i, proj := range m.projects {
			// Truncate project name to fit inner width minus the prefix (2 chars)
			line := truncateString(proj.ProjectName, leftInnerWidth-2)
			if i == m.projectCursor {
				if m.activePane == paneProjects {
					leftSb.WriteString(styleSelected.Width(leftInnerWidth).Render("> "+line) + "\n")
				} else {
					leftSb.WriteString(styleNormal.Background(colorDarkGray).Width(leftInnerWidth).Render("  "+line) + "\n")
				}
			} else {
				leftSb.WriteString(styleNormal.Render("  "+line) + "\n")
			}
		}
	}

	leftStyle := styleBox
	if m.activePane == paneProjects {
		leftStyle = styleBoxActive
	}
	leftPanel := leftStyle.Width(leftWidth).Height(heightVal).Render(leftSb.String())

	// Tasks list (Right)
	var rightSb strings.Builder
	var projName string
	if len(m.projects) > 0 {
		projName = m.projects[m.projectCursor].ProjectName
	}
	// Truncate header if project name is very long
	truncatedProjName := truncateString(projName, max(10, rightInnerWidth-8))
	rightSb.WriteString(styleHeader.Width(rightInnerWidth).Render("TASKS ("+truncatedProjName+")") + "\n")

	if len(m.projects) == 0 {
		rightSb.WriteString(styleMuted.Render("Select or create a project."))
	} else {
		tasks := m.projects[m.projectCursor].Tasks
		if len(tasks) == 0 {
			rightSb.WriteString(styleMuted.Render("No tasks in this project.\nPress 'c' to create a task."))
		} else {
			for i, task := range tasks {
				durStr := formatDuration(task.TotalDuration)
				// Calculate dynamic width for task name
				// Subtract prefix (2 chars), spacing (2 chars), and duration string length
				taskNameMaxLen := max(10, rightInnerWidth-2-2-len(durStr))
				taskNameTruncated := truncateString(task.TaskName, taskNameMaxLen)

				line := fmt.Sprintf("%-*s  %s", taskNameMaxLen, taskNameTruncated, durStr)
				if i == m.taskCursor {
					if m.activePane == paneTasks {
						rightSb.WriteString(styleSelected.Width(rightInnerWidth).Render("> "+line) + "\n")
					} else {
						rightSb.WriteString(styleNormal.Background(colorDarkGray).Width(rightInnerWidth).Render("  "+line) + "\n")
					}
				} else {
					rightSb.WriteString(styleNormal.Render("  "+line) + "\n")
				}
			}
		}
	}

	rightStyle := styleBox
	if m.activePane == paneTasks {
		rightStyle = styleBoxActive
	}
	rightPanel := rightStyle.Width(rightWidth).Height(heightVal).Render(rightSb.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
}

func (m mainModel) drawHistory() string {
	var body strings.Builder
	widthVal := max(50, m.width-6)
	heightVal := max(8, m.height-10)
	innerWidth := widthVal - 6

	body.WriteString(styleHeader.Width(innerWidth).Render("RECENT HISTORY") + "\n")

	if len(m.history) == 0 {
		body.WriteString(styleMuted.Render("No history entries yet. Start tracking to record some time!"))
	} else {
		// Dynamically calculate history column widths to prevent wrapping
		durColWidth := 10
		startColWidth := 16
		spacing := 2
		// Available for Project and Task:
		remaining := innerWidth - durColWidth - startColWidth - (spacing * 3) - 2 // -2 for prefix space
		if remaining < 12 {
			remaining = 12
		}
		projColWidth := remaining / 2
		taskColWidth := remaining - projColWidth

		rowFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%-%ds", projColWidth, taskColWidth, startColWidth, durColWidth)

		// Table Header
		body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorPurple).Render(
			fmt.Sprintf(rowFmt, "PROJECT", "TASK", "STARTED AT", "DURATION"),
		) + "\n")
		body.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Render(strings.Repeat("─", innerWidth)) + "\n")

		// Table Content
		endIndex := min(len(m.history), m.historyOffset+m.historyLimit)
		for i := m.historyOffset; i < endIndex; i++ {
			h := m.history[i]
			proj := truncateString(h.ProjectName, projColWidth)
			task := truncateString(h.TaskName, taskColWidth)
			start := h.StartAt.Format("2006-01-02 15:04")
			dur := formatDuration(h.Duration)
			if h.StopAt == nil {
				dur = "Running..."
			}

			line := fmt.Sprintf(rowFmt, proj, task, start, dur)
			if i == m.historyCursor {
				body.WriteString(styleSelected.Width(innerWidth).Render(line) + "\n")
			} else {
				if h.StopAt == nil {
					body.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Render(line) + "\n")
				} else {
					body.WriteString(styleNormal.Render(line) + "\n")
				}
			}
		}

		// Scroll indicator
		if len(m.history) > m.historyLimit {
			body.WriteString(styleMuted.Render(fmt.Sprintf("  Row %d to %d of %d (Use ↑/↓ keys to scroll)", m.historyOffset+1, endIndex, len(m.history))))
		}
	}

	return styleBoxActive.Height(heightVal).Width(widthVal).Render(body.String())
}

func (m mainModel) drawModal() string {
	var title string
	var content strings.Builder

	switch m.modal {
	case modalCreateProject:
		title = "Create New Project"
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to confirm  •  Esc to cancel"))
	case modalRenameProject:
		title = "Rename Project"
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to confirm  •  Esc to cancel"))
	case modalCreateTask:
		title = "Create New Task"
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to confirm  •  Esc to cancel"))
	case modalRenameTask:
		title = "Rename Task"
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to confirm  •  Esc to cancel"))
	case modalStartSelectProj:
		title = "Select Project to Track Task"
		var sb strings.Builder
		limit := 8
		offset := 0
		if m.projectSelectCursor >= limit {
			offset = m.projectSelectCursor - limit + 1
		}
		end := min(len(m.projectSelectNames), offset+limit)
		for i := offset; i < end; i++ {
			proj := m.projectSelectNames[i]
			prefix := "  "
			if i == m.projectSelectCursor {
				prefix = "> "
			}
			if i == m.projectSelectCursor {
				sb.WriteString(styleSelected.Width(35).Render(prefix+proj) + "\n")
			} else {
				sb.WriteString(styleNormal.Render(prefix+proj) + "\n")
			}
		}
		content.WriteString(sb.String())
		content.WriteString("\n" + styleMuted.Render("↑/↓ to navigate  •  Enter to select  •  Esc to cancel"))
	case modalStartSelectTask:
		title = fmt.Sprintf("Select Task in %q", m.selectedProj)
		var sb strings.Builder
		limit := 8
		offset := 0
		if m.taskSelectCursor >= limit {
			offset = m.taskSelectCursor - limit + 1
		}
		end := min(len(m.taskSelectNames), offset+limit)
		for i := offset; i < end; i++ {
			task := m.taskSelectNames[i]

			var lineStyle lipgloss.Style
			if i == m.taskSelectCursor {
				lineStyle = styleSelected.Width(35)
			} else if task == "[ + Create & Start New Task... ]" {
				lineStyle = lipgloss.NewStyle().Foreground(colorPink)
			} else {
				lineStyle = styleNormal
			}

			prefix := "  "
			if i == m.taskSelectCursor {
				prefix = "> "
			}

			sb.WriteString(lineStyle.Render(prefix+task) + "\n")
		}
		content.WriteString(sb.String())
		content.WriteString("\n" + styleMuted.Render("↑/↓ to navigate  •  Enter to select  •  Esc to go back"))
	case modalStartCreateTaskName:
		title = fmt.Sprintf("Create & Start Task in %q", m.selectedProj)
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to start  •  Esc to cancel"))
	case modalConfirmDeleteProj:
		title = "⚠️ Delete Project"
		projName := m.projects[m.projectCursor].ProjectName
		content.WriteString(fmt.Sprintf("Are you sure you want to delete project %q?\n", projName))
		content.WriteString(styleWarning.Render("This will delete ALL tasks and history under this project.") + "\n\n")
		content.WriteString(styleNormal.Render("Press [Y] to delete  •  [N] or Esc to cancel"))
	case modalConfirmDeleteTask:
		title = "⚠️ Delete Task"
		taskName := m.projects[m.projectCursor].Tasks[m.taskCursor].TaskName
		content.WriteString(fmt.Sprintf("Are you sure you want to delete task %q?\n", taskName))
		content.WriteString(styleWarning.Render("This will delete ALL history logs for this task.") + "\n\n")
		content.WriteString(styleNormal.Render("Press [Y] to delete  •  [N] or Esc to cancel"))
	case modalConfirmDeleteEntry:
		title = "⚠️ Delete Time Entry"
		h := m.history[m.historyCursor]
		content.WriteString(fmt.Sprintf("Delete entry for %q (%s)?\n\n", h.TaskName, formatDuration(h.Duration)))
		content.WriteString(styleNormal.Render("Press [Y] to delete  •  [N] or Esc to cancel"))
	case modalUpdateEntryTime:
		title = "Update Time Entry Duration"
		h := m.history[m.historyCursor]
		content.WriteString(fmt.Sprintf("Task:    %s\nProject: %s\n\n", h.TaskName, h.ProjectName))
		content.WriteString(m.input.View() + "\n\n")
		content.WriteString(styleMuted.Render("Enter to confirm  •  Esc to cancel"))

	case modalReportSelectProj:
		title = "Filter Report by Project"
		var sb strings.Builder
		limit := 8
		offset := 0
		if m.projectSelectCursor >= limit {
			offset = m.projectSelectCursor - limit + 1
		}
		end := min(len(m.projectSelectNames), offset+limit)
		for i := offset; i < end; i++ {
			proj := m.projectSelectNames[i]
			prefix := "  "
			if i == m.projectSelectCursor {
				prefix = "> "
			}
			if i == m.projectSelectCursor {
				sb.WriteString(styleSelected.Width(35).Render(prefix+proj) + "\n")
			} else {
				sb.WriteString(styleNormal.Render(prefix+proj) + "\n")
			}
		}
		content.WriteString(sb.String())
		content.WriteString("\n" + styleMuted.Render("↑/↓ to navigate  •  Enter to select  •  Esc to cancel"))

	case modalReportSelectTime:
		title = "Filter Report by Time"
		var sb strings.Builder
		for i, opt := range m.projectSelectNames {
			prefix := "  "
			if i == m.projectSelectCursor {
				prefix = "> "
			}
			if i == m.projectSelectCursor {
				sb.WriteString(styleSelected.Width(35).Render(prefix+opt) + "\n")
			} else {
				sb.WriteString(styleNormal.Render(prefix+opt) + "\n")
			}
		}
		content.WriteString(sb.String())
		content.WriteString("\n" + styleMuted.Render("↑/↓ to navigate  •  Enter to select  •  Esc to cancel"))

	case modalReportSelectGroup:
		title = "Group Report"
		var sb strings.Builder
		for i, opt := range m.projectSelectNames {
			prefix := "  "
			if i == m.projectSelectCursor {
				prefix = "> "
			}
			if i == m.projectSelectCursor {
				sb.WriteString(styleSelected.Width(35).Render(prefix+opt) + "\n")
			} else {
				sb.WriteString(styleNormal.Render(prefix+opt) + "\n")
			}
		}
		content.WriteString(sb.String())
		content.WriteString("\n" + styleMuted.Render("↑/↓ to navigate  •  Enter to select  •  Esc to cancel"))
	}

	body := fmt.Sprintf("%s\n\n%s", lipgloss.NewStyle().Bold(true).Foreground(colorPink).Render(title), content.String())
	heightVal := max(8, m.height-10)
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(max(40, m.width-6)).
		Height(heightVal).
		Render(styleModal.Render(body))
}

func (m mainModel) drawHelp() string {
	var keys []string

	addKey := func(k, desc string) {
		keys = append(keys, styleHelpKey.Render(k)+" "+styleHelpDesc.Render(desc))
	}

	addKey("1-4/tab", "Switch Tab")

	switch m.activeTab {
	case tabDashboard:
		if m.activeTask != nil {
			addKey("s", "Stop Task")
		} else {
			addKey("n", "New Task")
		}
	case tabProjects:
		addKey("↑/↓", "Navigate")
		addKey("←/→", "Focus Pane")
		addKey("c", "Create")
		addKey("e", "Rename")
		if len(m.projects) > 0 {
			addKey("d/del", "Delete")
			if m.activePane == paneTasks && len(m.projects[m.projectCursor].Tasks) > 0 {
				addKey("enter", "Start Track")
			}
		}
	case tabHistory:
		addKey("↑/↓", "Scroll")
		if len(m.history) > 0 {
			addKey("u", "Update Time")
			addKey("d/del", "Delete Entry")
			addKey("enter", "Restart Track")
		}
	case tabReport:
		addKey("↑/↓", "Scroll")
		addKey("p", "Filter Project")
		addKey("t", "Filter Time")
		addKey("g", "Group By")
		addKey("r", "Reset Filters")
	}

	addKey("q", "Quit")

	return strings.Join(keys, "  •  ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateString(s string, l int) string {
	if len(s) <= l {
		return s
	}
	if l <= 3 {
		return s[:l]
	}
	return s[:l-3] + "..."
}

func (m mainModel) drawReport() string {
	var body strings.Builder
	widthVal := max(50, m.width-6)
	heightVal := max(8, m.height-10)
	innerWidth := widthVal - 8

	// 1. Controls/Header
	body.WriteString(styleHeader.Width(innerWidth).Render("REPORT & ANALYTICS") + "\n")

	// Current filters status (descriptive names, safely truncated to prevent wrapping)
	projFilterStr := "[ All Projects ]"
	if m.reportProjFilter != "" {
		projFilterStr = fmt.Sprintf("[ Project: %s ]", truncateString(m.reportProjFilter, 25))
	}

	timeFilterStr := "[ All Time ]"
	switch m.reportTimeFilter {
	case reportTimeToday:
		timeFilterStr = "[ Today ]"
	case reportTimeWeek:
		timeFilterStr = "[ This Week ]"
	case reportTimeMonth:
		timeFilterStr = "[ This Month ]"
	}

	groupFilterStr := "[ No Grouping ]"
	switch m.reportGroupFilter {
	case "day":
		groupFilterStr = "[ Group: Day ]"
	case "week":
		groupFilterStr = "[ Group: Week ]"
	case "month":
		groupFilterStr = "[ Group: Month ]"
	case "year":
		groupFilterStr = "[ Group: Year ]"
	}

	filterBar := fmt.Sprintf("  Filter: %s  %s  %s",
		lipgloss.NewStyle().Foreground(colorCyan).Render(projFilterStr),
		lipgloss.NewStyle().Foreground(colorCyan).Render(timeFilterStr),
		lipgloss.NewStyle().Foreground(colorCyan).Render(groupFilterStr),
	)
	body.WriteString(filterBar + "\n")
	body.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Render("  "+strings.Repeat("─", max(1, innerWidth-2))) + "\n")

	if len(m.reportLines) == 0 {
		body.WriteString("\n" + styleMuted.Render("  No time entries match the selected filters.") + "\n")
	} else {
		// Table Content with scrolling
		// Note: the first two lines of m.reportLines are the table headers.
		if len(m.reportLines) >= 2 {
			body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorPurple).Render(m.reportLines[0].text) + "\n")
			body.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Render(m.reportLines[1].text) + "\n")
		}

		// Now draw the scrollable content starting from index 2
		contentLines := m.reportLines[2:]
		limit := m.reportLimit - 2
		if limit < 3 {
			limit = 3
		}

		endIndex := min(len(contentLines), m.reportOffset+limit)
		for i := m.reportOffset; i < endIndex; i++ {
			line := contentLines[i]
			if line.text == "" {
				body.WriteString("\n")
			} else {
				if i == m.reportCursor {
					body.WriteString(styleSelected.Width(innerWidth).Render(line.text) + "\n")
				} else if line.isTotal {
					body.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render(line.text) + "\n")
				} else {
					body.WriteString(styleNormal.Render(line.text) + "\n")
				}
			}
		}

		// Scroll indicator
		if len(contentLines) > limit {
			body.WriteString("\n" + styleMuted.Render(fmt.Sprintf("  Row %d to %d of %d (Use ↑/↓ keys to scroll)", m.reportOffset+1, endIndex, len(contentLines))))
		}
	}

	return styleBoxActive.Height(heightVal).Width(widthVal).Render(body.String())
}

func (m mainModel) generateReportLines(rows []model.ReportRow, innerWidth int) []reportLine {
	if len(rows) == 0 {
		return nil
	}

	hasDate := false
	for _, r := range rows {
		if r.Date != "" {
			hasDate = true
			break
		}
	}

	durColWidth := 10
	dateColWidth := 0
	if hasDate {
		dateColWidth = 10
	}

	numSpacers := 2
	if hasDate {
		numSpacers = 3
	}
	spacing := 2

	remaining := innerWidth - durColWidth - dateColWidth - (numSpacers * spacing) - 2
	if remaining < 12 {
		remaining = 12
	}
	projColWidth := remaining / 2
	taskColWidth := remaining - projColWidth

	var rowFmt string
	if hasDate {
		rowFmt = fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%-%ds", projColWidth, taskColWidth, dateColWidth, durColWidth)
	} else {
		rowFmt = fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds", projColWidth, taskColWidth, durColWidth)
	}

	projHeader := truncateString("PROJECT", projColWidth)
	taskHeader := truncateString("TASK", taskColWidth)

	var lines []reportLine
	if hasDate {
		dateHeader := truncateString("DATE", dateColWidth)
		durHeader := truncateString("DURATION", durColWidth)
		lines = append(lines, reportLine{
			text:     fmt.Sprintf(rowFmt, projHeader, taskHeader, dateHeader, durHeader),
			isHeader: true,
		})
	} else {
		durHeader := truncateString("DURATION", durColWidth)
		lines = append(lines, reportLine{
			text:     fmt.Sprintf(rowFmt, projHeader, taskHeader, durHeader),
			isHeader: true,
		})
	}
	lines = append(lines, reportLine{
		text:     "  " + strings.Repeat("─", max(1, innerWidth-2)),
		isHeader: true,
	})

	var currentProject string
	var projectTotal time.Duration
	var grandTotal time.Duration

	for i, r := range rows {
		if currentProject != "" && r.ProjectName != currentProject {
			subtotalStr := formatDuration(projectTotal)
			projLabel := truncateString(currentProject+" Total", projColWidth)
			if hasDate {
				lines = append(lines, reportLine{
					text:    fmt.Sprintf(rowFmt, projLabel, "", "", subtotalStr),
					isTotal: true,
				})
			} else {
				lines = append(lines, reportLine{
					text:    fmt.Sprintf(rowFmt, projLabel, "", subtotalStr),
					isTotal: true,
				})
			}
			lines = append(lines, reportLine{text: ""}) // spacer
			projectTotal = 0
		}

		currentProject = r.ProjectName
		projectTotal += r.Duration
		grandTotal += r.Duration

		projStr := truncateString(r.ProjectName, projColWidth)
		taskStr := truncateString(r.TaskName, taskColWidth)
		durStr := formatDuration(r.Duration)

		if hasDate {
			dateStr := truncateString(r.Date, dateColWidth)
			lines = append(lines, reportLine{
				text: fmt.Sprintf(rowFmt, projStr, taskStr, dateStr, durStr),
			})
		} else {
			lines = append(lines, reportLine{
				text: fmt.Sprintf(rowFmt, projStr, taskStr, durStr),
			})
		}

		if i == len(rows)-1 {
			subtotalStr := formatDuration(projectTotal)
			projLabel := truncateString(currentProject+" Total", projColWidth)
			if hasDate {
				lines = append(lines, reportLine{
					text:    fmt.Sprintf(rowFmt, projLabel, "", "", subtotalStr),
					isTotal: true,
				})
			} else {
				lines = append(lines, reportLine{
					text:    fmt.Sprintf(rowFmt, projLabel, "", subtotalStr),
					isTotal: true,
				})
			}
		}
	}

	if grandTotal != projectTotal || currentProject == "" {
		lines = append(lines, reportLine{text: ""}) // spacer
		grandTotalStr := formatDuration(grandTotal)
		grandTotalLabel := truncateString("Grand Total", projColWidth)
		if hasDate {
			lines = append(lines, reportLine{
				text:    fmt.Sprintf(rowFmt, grandTotalLabel, "", "", grandTotalStr),
				isTotal: true,
			})
		} else {
			lines = append(lines, reportLine{
				text:    fmt.Sprintf(rowFmt, grandTotalLabel, "", grandTotalStr),
				isTotal: true,
			})
		}
	}

	return lines
}
