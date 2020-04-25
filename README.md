# Noise Gate

[![Build Status](https://travis-ci.org/go-noisegate/noisegate.svg?branch=master)](https://travis-ci.org/go-noisegate/noisegate)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-noisegate/noisegate)](https://goreportcard.com/report/github.com/go-noisegate/noisegate)

Noise Gate is the Golang test runner to get faster test results.

It selects the tests affected by your recent edits and run them using `go test`.

## Prerequisites

* Go 1.13 or later

## Quickstart

You usually use this tool via the editor plugin. So the best quickstart document depends on your editor:
* [emacs](https://github.com/go-noisegate/noisegate.el)
* [vscode](https://github.com/go-noisegate/vscode-go-noisegate)

(If your favorite editor is not here, please consider writing the plugin for your editor!)

The document below assumes you use this tool directly, but it's not usual.

--

This quickstart shows you how to use the Noise Gate to get faster test results.

Usually your editor plugin sends the recent edits and the test requests to the server (`gated`). In this quickstart, we are going to send them by ourselves using the cli (`gate`).

### Set up

1. Install the server (`gated`) and cli (`gate`):

   ```sh
   $ go get -u github.com/go-noisegate/noisegate/cmd/gate && go get -u github.com/go-noisegate/noisegate/cmd/gated
   ```

2. Run the server (`gated`) if it's not running yet.

   ```sh
   $ gated
   ```

3. Download the quickstart repository.

   ```sh
   $ go get -u github.com/go-noisegate/quickstart
   ```

### Run your tests

Let's assume you just implemented some [functions](https://github.com/go-noisegate/quickstart/blob/master/math.go) (`SlowAdd` and `SlowSub`) and [tests](https://github.com/go-noisegate/quickstart/blob/master/math_test.go) (`TestSlowAdd`, `TestSlowAdd_Overflow` and `TestSlowSub`) at the `quickstart` repository.

1. Run all the tests

   First, check if all the tests are passed. Run the `gate test` command at the repository root.

   ```sh
   $ cd $GOPATH/src/github.com/go-noisegate/quickstart
   $ gate test -bypass . -- -v
   Run all tests:
   === RUN   TestSlowAdd
   --- PASS: TestSlowAdd (1.01s)
   === RUN   TestSlowAdd_Overflow
   --- PASS: TestSlowAdd_Overflow (1.01s)
   === RUN   TestSlowSub
   --- FAIL: TestSlowSub (1.00s)
       math_test.go:22: wrong result: 2
   FAIL
   FAIL    github.com/go-noisegate/quickstart     3.019s
   FAIL
   ```

   * One failed test. We will fix this soon.
   * With the `-bypass` option, the tool runs all the tests regardless of the recent changes.
   * The `.` arg specifies the directory to run the tests.
   * The args after `--` is passed to `go test`. `-v` is passed in this example.

2. Change the code

   To fix the failed test, open the `math.go` and change [the `SlowSub` function](https://github.com/go-noisegate/quickstart/blob/master/math.go#L12). `return a + b` at the line 12 should be `return a - b`.

3. Hint the change

   Run the `gate hint` to notify the server of the changed filename and position.

   ```sh
   $ gate hint math.go:#176
   ```

   `math.go` is the changed filename and `176` is the byte offset. The offset points to the `-` character at the line 12. Usually your editor plugin calculates this offset.

4. Run the tests affected by the recent changes

   Let's check if the test is fixed. Run the `gate test` again.

   ```sh
   $ gate test . -- -v
   Changed: [SlowSub]
   === RUN   TestSlowSub
   --- PASS: TestSlowSub (1.00s)
   PASS
   ok      github.com/go-noisegate/quickstart     1.006s
   ```

   * Without the `-bypass` option, the tool runs the tests affected by the recent changes.
   * The recent changes are listed at the `Changed: [SlowSub]` line. The list is cleared when all the tests are passed.
   * Based on the recent changes, the tool selects and runs only the `TestSlowSub` test.
   * *You get the faster test results (`3.019s` -> `1.006s`)!*

## How-to guides

### Pass options to `go test`

The args after `--` is passed to `go test`. For example, the command below passes the build tags.

```
$ gate test . -- -v -tags tags,list
```

### Run all tests

With the -bypass option, the tool runs all the tests regardless of the recent changes.

```
$ gate test -bypass . -- -v
```

## How it works

See [DEVELOPMENT.md](https://github.com/go-noisegate/noisegate/blob/master/DEVELOPMENT.md).
