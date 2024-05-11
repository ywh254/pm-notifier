package main

type Config struct {
	Login        Login        `mapstructure:"login"`
	Project      Project      `mapstructure:"project"`
	Notification Notification `mapstructure:"notification"`
}

type Login struct {
	LoginType string `mapstructure:"login_type"`
	Url       string `mapstructure:"url"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Cookie    string `mapstructure:"cookie"`
	Token     string `mapstructure:"token"`
}

type Project struct {
	TaskIterationListUrl string       `mapstructure:"task_iteration_list_url"`
	TaskListUrl          string       `mapstructure:"task_list_url"`
	TaskListMaxSize      uint         `mapstructure:"task_list_max_size"`
	TaskDeadlineFormat   string       `mapstructure:"task_deadline_format"`
	FieldMapping         FieldMapping `mapstructure:"field_mapping"`
}

type FieldMapping struct {
	ExecutorName string `mapstructure:"executor_name"`
	TaskName     string `mapstructure:"task_name"`
	TaskDone     string `mapstructure:"task_done"`
	TaskDeadline string `mapstructure:"task_deadline"`
}

type Notification struct {
	Type                   uint   `mapstructure:"type"`
	Webhook                string `mapstructure:"webhook"`
	Token                  string `mapstructure:"token"`
	Key                    string `mapstructure:"key"`
	Secret                 string `mapstructure:"secret"`
	StartTime              string `mapstructure:"start_time"`
	ExcludeDate            uint   `mapstructure:"exclude_date"`
	DateUrl                string `mapstructure:"date_url"`
	Title                  string `mapstructure:"title"`
	NowTaskListDesc        string `mapstructure:"now_task_list_desc"`
	NotDoneTaskListDesc    string `mapstructure:"not_done_task_list_desc"`
	NoDeadlineTaskListDesc string `mapstructure:"no_deadline_task_list_desc"`
}
