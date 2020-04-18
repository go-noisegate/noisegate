# Development

## How it works

### Find test functions affected by recent changes

The current approach is:
* The editor plugin updates the list of the changes while a user edits the files.
* When the `gate hint` command is called with the list of the changes, the server updates its internal list of the changes. The server maintains the change list for each directory.
* When the `gate test` command is called, the server parses go files in the package and finds the affected test functions. The list of changes associated with the directory are cleared.

The detail of the last part is ([code here](https://github.com/ks888/noisegate/blob/master/server/dependency.go)):
1. Parse go files in the package. Dependent packages are not parsed.
2. For each element of the change list:

   2-1. Finds the top-level declaration which encloses the specified change. Typically it's function declaration.

   2-2a. If the top-level declaration is the test function, it's affected.

   2-2b. Otherwise, find the test functions which uses the declared element. These test functions are affected.

Some pros and cons:
* Lightweight. Parsing files can be heavy, but the tool does that only when the `gate test` is called.
* Less precise. The content of the file may have changed dramatically since the `gate hint` is called and so the tool may consider the wrong test function as 'affected'.
* Easy to understand. As described above, the tool does the simple analysis and a user can easily understand why some tests are executed. For the same reason, the implementation can be less buggy.

If you come up with another approach or some improvements, please create an issue!
