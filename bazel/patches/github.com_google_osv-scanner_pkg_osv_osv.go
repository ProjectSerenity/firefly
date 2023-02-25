diff --git a/pkg/osv/osv.go b/pkg/osv/osv.go
index c28708a..4c52481 100644
--- a/pkg/osv/osv.go
+++ b/pkg/osv/osv.go
@@ -8,7 +8,6 @@ import (
 	"net/http"
 	"time"
 
-	"github.com/google/osv-scanner/pkg/lockfile"
 	"github.com/google/osv-scanner/pkg/models"
 )
 
@@ -85,18 +84,6 @@ func MakePURLRequest(purl string) *Query {
 	}
 }
 
-func MakePkgRequest(pkgDetails lockfile.PackageDetails) *Query {
-	return &Query{
-		Version: pkgDetails.Version,
-		// API has trouble parsing requests with both commit and Package details filled ins
-		// Commit:  pkgDetails.Commit,
-		Package: Package{
-			Name:      pkgDetails.Name,
-			Ecosystem: string(pkgDetails.Ecosystem),
-		},
-	}
-}
-
 // From: https://stackoverflow.com/a/72408490
 func chunkBy[T any](items []T, chunkSize int) [][]T {
 	chunks := make([][]T, 0, (len(items)/chunkSize)+1)
