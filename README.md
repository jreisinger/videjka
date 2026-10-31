Videjka finds video files with human-created names inside a [OneDrive] folder
and optionally copies them.

    go run github.com/jreisinger/videjka@latest -h
	# or
	go install github.com/jreisinger/videjka@latest
	videjka -h

Ctrl-C or SIGTERM stops discovery and waits for active copies to finish.
A second signal forces exit and can leave partially copied files.
