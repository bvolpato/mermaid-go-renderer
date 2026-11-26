// This file ports no upstream file. It reproduces the order in which a
// browser runs asynchronous code: promises, microtasks and the measure queue
// of fastdom.

package render

import "iter"

// This file reproduces the order in which a browser runs asynchronous code.
//
// Upstream starts the drawing of many nodes at once:
//
//	await Promise.all(nodes.map(async (node) => { await insertNode(...) }))
//
// Each callback runs until its first await. Then the callbacks continue in
// the order of the microtask queue. So a shape handler that has no await
// draws before the handlers that wait for a label, and a label that needs
// more awaits continues after the others. The order decides which shape
// gets which numbers of Math.random, so it shows in the output.
//
// A port keeps the order with three calls:
//
//   - ctx.All(n, fn) for await Promise.all(items.map(async ...)).
//   - ctx.Await() where upstream has an await.
//   - ctx.AwaitMeasure() for await fastdom.measure(...).
//
// Convention: a Go function that ports an async function calls ctx.Await()
// once before it returns, for the await of its caller. The caller adds
// nothing for that call. A caller adds ctx.Await() itself only when it
// awaits something else, for example a value or a function that is not
// async.
//
// Outside ctx.All the calls do nothing: sequential code needs no scheduler.
//
// Each task of ctx.All is a coroutine (iter.Pull): it has a stack of its
// own, and the scheduler switches to it and back without the scheduler of
// the Go runtime. Exactly one of them runs at a time, so the result is
// deterministic and the tasks need no locks. A switch through channels,
// which the first version of this file used, wakes another thread for each
// switch: that was a quarter of the CPU time of a flowchart.

// asyncTask is one callback of All.
type asyncTask struct {
	// resume gives control to the task until it parks or ends.
	resume func() (struct{}, bool)
	// park gives control back to the scheduler. The task calls it.
	park func(struct{}) bool
	// delay is the number of extra turns of the queue before the task
	// continues.
	delay int
}

// asyncScheduler is the microtask queue of one render.
type asyncScheduler struct {
	micro     []*asyncTask
	microHead int
	// reads are the tasks that wait for the next flush of fastdom.
	reads []*asyncTask
	// timers are the tasks that wait for a timer.
	timers    []*asyncTask
	timerHead int
	cur       *asyncTask
	// flushQueued is true while a flush of fastdom is in the queue.
	flushQueued bool
	// check stops the render when its context is done.
	check func()
}

func (s *asyncScheduler) pushMicro(t *asyncTask) {
	if s.microHead == len(s.micro) {
		s.micro = s.micro[:0]
		s.microHead = 0
	} else if s.microHead > 0 && len(s.micro) == cap(s.micro) {
		n := copy(s.micro, s.micro[s.microHead:])
		clear(s.micro[n:])
		s.micro = s.micro[:n]
		s.microHead = 0
	}
	s.micro = append(s.micro, t)
}

func (s *asyncScheduler) pushTimer(t *asyncTask) {
	if s.timerHead == len(s.timers) {
		s.timers = s.timers[:0]
		s.timerHead = 0
	} else if s.timerHead > 0 && len(s.timers) == cap(s.timers) {
		n := copy(s.timers, s.timers[s.timerHead:])
		clear(s.timers[n:])
		s.timers = s.timers[:n]
		s.timerHead = 0
	}
	s.timers = append(s.timers, t)
}

// flushMarker stands in the microtask queue for the flush of fastdom.
// Mermaid configures fastdom to flush from queueMicrotask.
var flushMarker = &asyncTask{}

func (c *Context) scheduler() *asyncScheduler {
	if c.sched == nil {
		c.sched = &asyncScheduler{check: c.CheckCanceled}
	}
	return c.sched
}

// run gives control to a task until it parks or ends.
func (s *asyncScheduler) run(t *asyncTask) {
	prev := s.cur
	s.cur = t
	t.resume()
	s.cur = prev
}

// park stops the current task until the scheduler runs it again.
func (s *asyncScheduler) park() {
	t := s.cur
	t.park(struct{}{})
	// A canceled render stops here, inside the task, so that every task
	// ends and no goroutine stays behind.
	if s.check != nil {
		s.check()
	}
}

// next takes the next task from the queue. It returns nil when nothing can
// run.
func (s *asyncScheduler) next() *asyncTask {
	for {
		if s.microHead == len(s.micro) {
			s.micro = s.micro[:0]
			s.microHead = 0
			if s.timerHead == len(s.timers) {
				s.timers = s.timers[:0]
				s.timerHead = 0
				return nil
			}
			// A timer is a task of its own: the microtasks that it starts
			// run before the next timer.
			tm := s.timers[s.timerHead]
			s.timers[s.timerHead] = nil
			s.timerHead++
			s.micro = append(s.micro, tm)
		}
		t := s.micro[s.microHead]
		s.micro[s.microHead] = nil
		s.microHead++
		if s.microHead == len(s.micro) {
			s.micro = s.micro[:0]
			s.microHead = 0
		}
		if t == flushMarker {
			// The flush runs every queued read. Each read resolves its
			// promise, so the readers continue in the order of their reads.
			s.flushQueued = false
			for _, r := range s.reads {
				s.pushMicro(r)
			}
			clear(s.reads)
			s.reads = s.reads[:0]
			continue
		}
		if t.delay > 0 {
			t.delay--
			s.pushMicro(t)
			continue
		}
		return t
	}
}

// Await is one await of upstream: the current task goes to the end of the
// microtask queue.
func (c *Context) Await() {
	c.AwaitN(1)
}

// AwaitN is n awaits in a row with nothing between them.
func (c *Context) AwaitN(n int) {
	s := c.sched
	if s == nil || s.cur == nil || n <= 0 {
		return
	}
	s.cur.delay = n - 1
	s.pushMicro(s.cur)
	s.park()
}

// AwaitMeasure is await fastdom.measure(fn): the task continues after the
// next flush of fastdom. Call it before the measurement.
func (c *Context) AwaitMeasure() {
	s := c.sched
	if s == nil || s.cur == nil {
		return
	}
	s.reads = append(s.reads, s.cur)
	if !s.flushQueued {
		s.flushQueued = true
		s.pushMicro(flushMarker)
	}
	s.park()
}

// AwaitTimeout is the wait for a setTimeout callback: the task continues
// after the microtask queue is empty.
func (c *Context) AwaitTimeout() {
	s := c.sched
	if s == nil || s.cur == nil {
		return
	}
	s.pushTimer(s.cur)
	s.park()
}

type allState struct {
	pending    int
	ownerWaits bool
	firstErr   error
	panicked   any
	hasPanic   bool
}

// All is await Promise.all(items.map(async (item, i) => fn(i))). It starts
// fn(0) to fn(n-1) in order, each up to its first await, then runs them to
// the end in the order of the microtask queue. It returns the first error
// in the order of completion. A panic in a task continues in the caller
// after every task has ended.
func (c *Context) All(n int, fn func(i int) error) error {
	if n == 0 {
		// await Promise.all([]) is one await.
		c.Await()
		return nil
	}
	if n == 1 && (c.sched == nil || c.sched.cur == nil) {
		return fn(0)
	}
	s := c.scheduler()
	owner := s.cur
	st := &allState{pending: n}
	tasks := make([]asyncTask, n)
	for i := 0; i < n; i++ {
		t := &tasks[i]
		idx := i
		// The task runs when the scheduler resumes it, and every task runs
		// to its end, so the coroutine needs no stop.
		t.resume, _ = iter.Pull(func(park func(struct{}) bool) {
			t.park = park
			defer func() {
				if p := recover(); p != nil && !st.hasPanic {
					st.hasPanic, st.panicked = true, p
				}
				st.pending--
				if st.pending == 0 && st.ownerWaits {
					// Promise.all resolves one turn after its last item,
					// and the await of the caller takes one more.
					owner.delay = 1
					s.pushMicro(owner)
				}
			}()
			if err := fn(idx); err != nil && st.firstErr == nil {
				st.firstErr = err
			}
		})
		s.run(t)
	}
	if owner == nil {
		for st.pending > 0 {
			t := s.next()
			if t == nil {
				panic("render: tasks of All wait and nothing can run")
			}
			s.run(t)
		}
	} else if st.pending > 0 {
		st.ownerWaits = true
		s.park()
	} else {
		c.AwaitN(2)
	}
	if st.hasPanic {
		panic(st.panicked)
	}
	return st.firstErr
}
