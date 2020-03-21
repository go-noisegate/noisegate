# Hornet

Hornet is the Golang test runner for the speedster.

The core features are:
* **Change-driven**: by the integration with your editor, hornet knows what changes you made. It runs the tests affected by these changes first.
* **Tuned for high-speed**: hornet implements some strategies to run the tests faster, including tests in parallel. You may disable these features for safety.

## Prerequisites

* Go 1.13 or later
* Linux or Mac OS X

## Quickstart

You usually run the hornet via the editor plugin. So check out the quickstart document of the plugin you installed:
* [emacs](https://github.com/ks888/hornet.el)
* [vscode](https://github.com/ks888/vscode-go-hornet)

(If your favorite editor is not here, please consider writing the plugin for your editor!)

The document below assumes you run the hornet directly, but it's not so common.

--

This quickstart shows you how to use hornet to help your coding.

### Set up

1. Hornet has the server program (`hornetd`) and client program (`hornet`). Install both:

   ```sh
   $ go get -u github.com/ks888/hornet/cmd/hornet && go get -u github.com/ks888/hornet/cmd/hornetd
   ```

2. Run the server program (`hornetd`) if it's not running yet.

   ```sh
   $ hornetd
   ```

3. Download the sample repository.

   ```sh
   $ go get -u github.com/ks888/hornet-tutorial
   ```

### Coding

Let's assume you just implemented some [functions](https://github.com/ks888/hornet-tutorial/blob/master/math.go) (`SlowAdd` and `SlowSub`) and [tests](https://github.com/ks888/hornet-tutorial/blob/master/math_test.go) (`TestSlowAdd`, `TestSlowAdd_Overflow` and `TestSlowSub`) in the `hornet-tutorial` repository.

1. Run your first tests

   Run the `hornet test` at the repository root. It runs all the tests in the package.

   ```sh
   $ cd $GOPATH/src/github.com/ks888/hornet-tutorial
   $ hornet test .   # absolute path is also ok
   Changed: []

   Run affected tests:

   Run other tests:
   === RUN   TestSlowAdd
   --- PASS: TestSlowAdd (1.01s)
   === RUN   TestSlowAdd_Overflow
   --- PASS: TestSlowAdd_Overflow (1.01s)
   === RUN   TestSlowSub
   --- FAIL: TestSlowSub (1.01s)
       math_test.go:22: wrong result: 2
   FAIL (1.03338981s)
   ```

   * `Changed` and `Run affected tests` are empty since we don't make any changes yet.
   * One failed test. We will fix this next.
   * The test time is `1.033450365s` because the tests run in parallel. When you run the same tests using `go test`, it takes about 3 seconds.

2. Fix the bug

   Open the `math.go` and fix [the `SlowSub` function](https://github.com/ks888/hornet-tutorial/blob/master/math.go#L12). `return a + b` at the line 12 should be `return a - b`.

3. Hint the change of the `math.go`

   Run the `hornet hint` to notify the hornet server of the changed filename and position.

   ```sh
   $ hornet hint math.go:#176
   ```

   `176` is the byte offset and points to the `-` character at the line 12. Usually your editor plugin calculates this offset.

4. Run the tests again

   When you run the `hornet test` again, the previous hint is considered.

   ```sh
   $ hornet test .
   Changed: [SlowSub]

   Run affected tests:
   === RUN   TestSlowSub
   --- PASS: TestSlowSub (1.01s)

   Run other tests:
   === RUN   TestSlowAdd_Overflow
   --- PASS: TestSlowAdd_Overflow (1.00s)
   === RUN   TestSlowAdd
   --- PASS: TestSlowAdd (1.01s)
   PASS (1.034054983s)
   ```

   *The tool knows you've changed the `SlowSub` function and runs affected tests (`TestSlowSub`) first.*

## How-to guides

### Run tests in sequence

Some tests fail when they are executed in parallel. You can use the `--parallel` or `-p` option to run them in sequence.

```
$ hornet test -p off [filename:#offset]
```

### Specify the build tags

It's same as `go test`.

```
$ hornet test --tags tags,list [filename:#offset]
```

