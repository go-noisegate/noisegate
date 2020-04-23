# Development

## How it works

### Find test functions affected by recent changes

The current approach:

```
1. Parses all the files in the directory.
2. For each change, find the affected test functions:
   2-1. Finds the function or general declaration which encloses the change.
   2-2a. If the declaration is the test function, the function is affected.
   2-2b. Otherwise, finds the entities which uses the declaration, by traversing the AST tree.
         (To detect the 'use', it simply compares the entity name with the declared name.)
         Then, check if the ascendant AST nodes of entities are the test function declaration. If so, the function is affected.
```

[See the code](https://github.com/go-noisegate/noisegate/blob/master/server/dependency.go) for more details.

Some pros and cons:
* Lightweight
   * Parsing the entire workspace can be very slow but we parse only the files in one directory. Usually it takes 10-20ms.
* Less false negative, more false positive
   * At the step 2-2b, we simply compare the name, but the name is not always unique. For example, `Calculator.Sum()` and `(*SimpleCalculator).Sum()` have the same method name, but its implementation may be diffirent (and if so, it's false positive).
   * The content of the file may have changed dramatically since the list of changes are sent to the server. The tool may consider the wrong test function as 'affected'.

If you know different approaches or some improvements, please create an issue!
