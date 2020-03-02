# Hornet

Hornet is the Golang test runner for the speedster, designed to minimize the time to find the failed test case.

The core features are:
* Change Oriented: by the integration with your editor, hornet knows what changes you made and runs the tests affected by these changes first.
* Tuned for Speed: hornet implements some strategies to run the tests faster, including tests in parallel and the package pre-building. You may disable these features for safety.

## Prerequisites

* Go 1.13 or later
* Linux or Mac OS X

## Install

Hornet has the server program (`hornetd`) and client program (`hornet`). Install both:

```sh
$ go get -u github.com/ks888/hornet/cmd/hornet && go get -u github.com/ks888/hornet/cmd/hornetd
```

Hornet works with your editor and so the next step depends on your editor:
* [emacs]()
* [vscode]()

(If your favorite editor is not here, please consider writing the plugin for your editor!)

## Quickstart

You usually run the hornet via the editor plugin. So check out the quickstart document of the plugin you installed:
* [emacs]()
* [vscode]()

The document below assumes you run the hornet directly, but it's not so common.

--

This quickstart shows you how to use hornet to help your coding.

### Set up

1. Run the server program (`hornetd`) if it's not running yet.

   ```sh
   $ hornetd
   ```

2. Download the sample repository.

   ```sh
   $ go get -u github.com/ks888/hornet-tutorial
   ```

### Coding

Let's assume you just implemented some [functions](https://github.com/ks888/hornet-tutorial/blob/master/math.go) (`SlowAdd` and `SlowSub`) and [tests](https://github.com/ks888/hornet-tutorial/blob/master/math_test.go) (`TestSlowAdd`, `TestSlowAdd_Overflow` and `TestSlowSub`) in the `hornet-tutorial` repository.

1. Run the tests

   Run the `hornet test` at the repository root. It runs all the tests in the package.

   ```sh
   $ hornet test .   # absolute path is also ok
   No important tests. Run all the tests:
   === RUN   TestSlowAdd
   --- PASS: TestSlowAdd (1.01s)
   === RUN   TestSlowAdd_Overflow
   --- PASS: TestSlowAdd_Overflow (1.01s)
   === RUN   TestSlowSub
   --- FAIL: TestSlowSub (1.00s)
       math_test.go:22: wrong result: 2
   FAIL (1.033450365s)
   ```

   Obviously there is one failed test.

2. Fix the bug

   Open the `math.go` and fix the `SlowSub` function. `return a + b` at the line 12 should be `return a - b`.

3. Hint the change of the `math.go`

   Run the `hornet hint` to notify the hornet server of the changed filename and position.

   ```sh
   hornet hint math.go:#176
   ```

   `176` is the byte offset and it points to the `-` character at the line you changed. This is bothersome, but usually the editor plugin runs this command automatically.

4. Run the tests again

   When you run the `hornet test` again, the previous hint is considered.

   ```sh
   $ hornet test .   # [filepath:#offset] is also ok
   Found important tests. Run them first:
   === RUN   TestSlowSub
   --- PASS: TestSlowSub (1.00s)

   Run other tests:
   === RUN   TestSlowAdd
   --- PASS: TestSlowAdd (1.00s)
   === RUN   TestSlowAdd_Overflow
   --- PASS: TestSlowAdd_Overflow (1.00s)
   PASS (1.033799777s)
   ```

   Based on the hint, hornet runs `TestSlowSub` first because it's affected by the previous change.

   Note that the total test time is `1.033799777s` here because the tests run in parallel. When you run the same tests using `go test`, it will take about 3 seconds.

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

