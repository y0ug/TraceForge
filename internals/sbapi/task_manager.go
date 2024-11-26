package sbapi

import (
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type Task struct {
	ID       cron.EntryID `json:"id"`
	Name     string       `json:"name"`
	Schedule string       `json:"schedule"`
	Status   string       `json:"status"`
	Enabled  bool         `json:"enabled"`
	Job      func()       `json:"-"`
}

type TaskManager struct {
	Cron      *cron.Cron
	Tasks     map[string]*Task
	TaskMutex sync.RWMutex
	Logger    *logrus.Logger
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		Cron:   cron.New(),
		Tasks:  make(map[string]*Task),
		Logger: logrus.New(),
	}
}

func (tm *TaskManager) Start() {
	tm.Cron.Start()
}

func (tm *TaskManager) Stop() {
	tm.Cron.Stop()
}

func (tm *TaskManager) AddTask(name string, schedule string, job func() error) (*Task, error) {
	tm.TaskMutex.Lock()
	defer tm.TaskMutex.Unlock()

	// Wrap the job to update the task's status during execution
	wrappedJob := func() {
		tm.TaskMutex.Lock()
		task, exists := tm.Tasks[name]
		if task.Status == "running" {
			tm.Logger.Errorf("Task %s already running", name)
			tm.TaskMutex.Unlock()
			return
		}
		if exists {
			task.Status = "running"
		}
		tm.TaskMutex.Unlock()

		_ = job() // Execute the actual job

		tm.TaskMutex.Lock()
		if exists {
			task.Status = "stopped"
		}
		tm.TaskMutex.Unlock()
	}

	// Schedule can be empty to run the job immediately
	var id cron.EntryID
	var err error

	id = -1
	if schedule != "" {
		id, err = tm.Cron.AddFunc(schedule, wrappedJob)
		if err != nil {
			return nil, err
		}
	}

	task := &Task{
		ID:       id,
		Name:     name,
		Schedule: schedule,
		Status:   "stopped",
		Enabled:  true,
		Job:      wrappedJob,
	}

	// Run the job immediately if schedule is empty
	if id == -1 {
		go task.Job()
	}
	tm.Tasks[name] = task
	return task, nil
}

func (tm *TaskManager) GetTask(name string) (*Task, bool) {
	tm.TaskMutex.Lock()
	defer tm.TaskMutex.Unlock()

	task, exists := tm.Tasks[name]
	if !exists {
		return nil, false
	}
	return task, true
}

func (tm *TaskManager) RemoveTask(name string) bool {
	tm.TaskMutex.Lock()
	defer tm.TaskMutex.Unlock()

	task, exists := tm.Tasks[name]
	if !exists {
		return false
	}
	tm.Cron.Remove(task.ID)
	delete(tm.Tasks, name)
	return true
}

func (tm *TaskManager) RunTask(name string) *Task {
	tm.TaskMutex.RLock()
	defer tm.TaskMutex.RUnlock()

	task, exists := tm.Tasks[name]
	if !exists {
		return nil
	}

	if task.Enabled {
		if task.Status == "stopped" {
			go func() {
				task.Job()
			}()
			task.Status = "starting"
		}
	}

	return task
}

func (tm *TaskManager) GetTasks() []*Task {
	tm.TaskMutex.RLock()
	defer tm.TaskMutex.RUnlock()

	var tasks []*Task
	for _, task := range tm.Tasks {
		tasks = append(tasks, task)
	}
	return tasks
}
