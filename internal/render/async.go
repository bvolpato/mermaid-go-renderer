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
	micro []*asyncTask
	// reads are the tasks that wait for the next flush of fastdom.
	reads []*asyncTask
	// timers are the tasks that wait for a timer.
	timers []*asyncTask
	cur    *asyncTask
	// flushQueued is true while a flush of fastdom is in the queue.
	flushQueued bool
	// check stops the render when its context is done.
	check func()
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
		if len(s.micro) == 0 {
			if len(s.timers) == 0 {
				return nil
			}
			// A timer is a task of its own: the microtasks that it starts
			// run before the next timer.
			s.micro = append(s.micro, s.timers[0])
			s.timers = s.timers[1:]
		}
		t := s.micro[0]
		s.micro = s.micro[1:]
		if t == flushMarker {
			// The flush runs every queued read. Each read resolves its
			// promise, so the readers continue in the order of their reads.
			s.flushQueued = false
			s.micro = append(s.micro, s.reads...)
			s.reads = nil
			continue
		}
		if t.delay > 0 {
			t.delay--
			s.micro = append(s.micro, t)
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
	s.micro = append(s.micro, s.cur)
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
		s.micro = append(s.micro, flushMarker)
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
	s.timers = append(s.timers, s.cur)
	s.park()
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
	s := c.scheduler()
	owner := s.cur
	pending := n
	ownerWaits := false
	var firstErr error
	var panicked any
	hasPanic := false
	for i := 0; i < n; i++ {
		t := &asyncTask{}
		// The task runs when the scheduler resumes it, and every task runs
		// to its end, so the coroutine needs no stop.
		t.resume, _ = iter.Pull(func(park func(struct{}) bool) {
			t.park = park
			defer func() {
				if p := recover(); p != nil && !hasPanic {
					hasPanic, panicked = true, p
				}
				pending--
				if pending == 0 && ownerWaits {
					// Promise.all resolves one turn after its last item,
					// and the await of the caller takes one more.
					owner.delay = 1
					s.micro = append(s.micro, owner)
				}
			}()
			if err := fn(i); err != nil && firstErr == nil {
				firstErr = err
			}
		})
		s.run(t)
	}
	if owner == nil {
		for pending > 0 {
			t := s.next()
			if t == nil {
				panic("render: tasks of All wait and nothing can run")
			}
			s.run(t)
		}
	} else if pending > 0 {
		ownerWaits = true
		s.park()
	} else {
		c.AwaitN(2)
	}
	if hasPanic {
		panic(panicked)
	}
	return firstErr
}
