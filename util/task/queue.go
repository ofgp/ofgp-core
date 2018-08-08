package task

// Note:
// 1. NOT thread-safe
// 2. The order of task execution is guaranteed to be the same as the add order.
type Queue struct {
	tasks []Task
}

func (q *Queue) Add(t Task) {
	q.tasks = append(q.tasks, t)
}

func (q *Queue) ExecAll() {
	for _, t := range q.tasks {
		t()
	}
	q.tasks = nil
}

func (q *Queue) Clear() {
	q.tasks = nil
}

func (q *Queue) Size() int {
	return len(q.tasks)
}
