package compare

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestJSON(t *testing.T) {
	for _, test := range []struct {
		description string
		a           []byte
		b           []byte
		expected    string
	}{
		{
			description: "Modified",
			a:           ReadFixture("fixtures/base.json"),
			b:           ReadFixture("fixtures/base_modified.json"),
			expected:    modified,
		},
		{
			description: "ModifiedWildcard",
			a:           ReadFixture("fixtures/base.json"),
			b:           ReadFixture("fixtures/base_modified_wildcard.json"),
			expected:    modifiedWildcard,
		},
		{
			description: "ModifiedWildcardObj",
			a:           ReadFixture("fixtures/base.json"),
			b:           ReadFixture("fixtures/base_modified_wildcard_obj.json"),
			expected:    modifiedWildcardObj,
		},
	} {
		c, err := JSON(test.a, test.b)
		if err != nil {
			t.Errorf("%s - expecting no error, got %s", test.description, err)
		} else if c.output != test.expected {
			t.Errorf("%s \nexpecting: %s\ngot: \n%s", test.description, test.expected, c.output)
		}
	}
}

func ReadFixture(path string) []byte {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("fixture file '%s' not found: %s", path, err))
	}
	return content
}

var modified = ` {
   "arr": [
     "arr0",
     21,
     {
       "num": 1,
-      "str": "pek3f"
+      "str": "changed"
     },
     [
       0,
-      "1"
+      "changed"
     ]
   ],
   "bool": true,
-  "null": null,
   "num_float": 39.39,
   "num_int": 13,
   "obj": {
     "arr": [
       17,
       "str",
       {
-        "str": "eafeb"
+        "str": "changed"
       }
     ],
-    "num": 19,
     "obj": {
-      "num": 14,
+      "num": 9999,
-      "str": "efj3"
+      "str": "changed"
     },
     "str": "bcded"
+    "new": "added"
   },
   "str": "abcde"
 }
`
var modifiedWildcard = ` {
   "arr": [
     "arr0",
     21,
     {
-      "num": 1,
+      "num": 5,
       "str": "pek3f"
     },
     [
       0,
       "1"
     ]
   ],
   "bool": true,
   "null": null,
   "num_float": 39.39,
-  "num_int": 13,
+  "num_int": 20,
   "obj": {
     "arr": [
       17,
       "str",
       {
         "str": "eafeb"
       }
     ],
-    "num": 19,
+    "num": 21,
     "obj": {
       "num": 14,
       "str": "efj3"
     },
     "str": "bcded"
   },
   "str": "abcde"
 }
`
var modifiedWildcardObj = ` {
   "arr": [
     "arr0",
     21,
     {
       "num": 1,
       "str": "pek3f"
     },
     [
       0,
       "1"
     ]
   ],
   "bool": true,
   "null": null,
   "num_float": 39.39,
   "num_int": 13,
   "obj": {
     "arr": [
       17,
       "str",
       {
         "str": "eafeb"
       }
     ],
     "num": 19,
     "obj": {
       "num": 14,
       "str": "efj3"
     },
     "str": "bcded"
   },
   "str": "abcde"
 }
`
