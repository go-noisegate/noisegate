# Noise Gate

Noise Gate is the Golang test runner with the noise reduction.

When you run the tests, the tool eliminates the unimportant tests for the *faster test results*.

[image]()

The tool is integrated with your editor plugin and knows the changes you made. When you run the tests, it selects the tests affected by recent changes.

[image]()

## Prerequisites

* Go 1.13 or later
* Linux or Mac OS X

## Quickstart

You usually run this tool via the editor plugin. So the best quickstart document depends on your editor:
* [emacs](https://github.com/ks888/noisegate.el)
* [vscode](https://github.com/ks888/vscode-go-noisegate)

(If your favorite editor is not here, please consider writing the plugin for your editor!)

The document below assumes you run this tool directly, but it's unusual.

--

This quickstart shows you how to use this tool to help your coding.

### Set up

1. The tool has the server program (`gated`) and client program (`gate`). Install both:

   ```sh
   $ go get -u github.com/ks888/noisegate/cmd/gate && go get -u github.com/ks888/noisegate/cmd/gated
   ```

2. Run the server program (`gated`) if it's not running yet.

   ```sh
   $ gated
   ```

3. Download the sample repository.

   ```sh
   $ go get -u github.com/ks888/noisegate-tutorial
   ```

### Coding

Let's assume you just implemented some [functions](https://github.com/ks888/noisegate-tutorial/blob/master/math.go) (`SlowAdd` and `SlowSub`) and [tests](https://github.com/ks888/noisegate-tutorial/blob/master/math_test.go) (`TestSlowAdd`, `TestSlowAdd_Overflow` and `TestSlowSub`) in the `noisegate-tutorial` repository.

1. Run your first test

   Run the `gate test` at the repository root. With the `-bypass` option, the tool simply runs all the tests in the package.

   ```sh
   $ cd $GOPATH/src/github.com/ks888/noisegate-tutorial
   $ gate -bypass -v test .   # absolute path is also ok
   TODO
   ```

   * One failed test. We will fix this next.
   * The `gate` command supports almost all the options of the `go test` command. In this example, we pass the `-v` option.

2. Fix the bug

   Open the `math.go` and fix [the `SlowSub` function](https://github.com/ks888/noisegate-tutorial/blob/master/math.go#L12). `return a + b` at the line 12 should be `return a - b`.

3. Hint the change of the `math.go`

   Run the `gate hint` to notify the server of the changed filename and position.

   ```sh
   $ gate hint math.go:#176
   ```

   `176` is the byte offset and points to the `-` character at the line 12. Usually your editor plugin calculates this offset.

4. Run the test again

   When you run the `gate test` again, the previous hint is considered.

   ```sh
   $ gate test .
   TODO
   ```

   *The tool knows you've changed the `SlowSub` function and runs affected tests (`TestSlowSub`) only.*

## How-to guides

### Specify the build tags

It's same as `go test`. Actually, the `gate` command supports almost all the options of the `go test` command.

```
$ gate test --tags tags,list [filename:#offset]
```

## How it works

See [DEVELOPMENT.md](https://github.com/ks888/noisegate/blob/master/DEVELOPMENT.md).
