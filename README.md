# gg-deployer - Go Git Deployer

This is a simple deployer written in go, using git.

# Usage:

```
gg-deployer [--debug] -c <config.json>
```

`config.json` is the configuration file. Its content is like:

```
{
    // how many worker go-routines to prepare for incoming deploy jobs
    "max_workers": 5, 

    // the configuration of HTTP server
    "http_server": {
        "host": "0.0.0.0", // which host to listen on
        "port": 8088       // which port to listen on
    },

    // define projects
    "projects": [
        {
            // Only github and gogs are supported now
            "type": "github",  

            // The github repo full name (case sensitive)
            "repo": "someuser/somerepo", 

            // The repo URL to pull the repo down.
            // It can be omitted if type is github. If omitted, default repo_url is git@github.com:<repo>.git
            "repo_url": "git@github.com:someuser/somerepo.git", 

            // The secret you enter in the push event configuration page.
            "secret": "!!EnterYourSecretHere!!",

            // Where to store the repo. 
            // It should be a private path. And you must NOT change its content.
            "store": "/path/to/store/the/repo",

            // Where to deploy the project. 
            "target": "/path/to/deploy/target",

            // Which branch to deploy(checkout).
            "branch": "master",

            // as which user to run the deploy command
            "run_as": "www",

            // script to run after checkout/pull done, in store directory.
            "post_checkout_script": "echo This is post checkout script",

            // script to run after rsync done, in target directory
            "post_rsync_script": "echo This is post rsync script",

            // override default deploy commands.
            // if `$exec` is not empty, only `$exec` will be executed. default checkout and rsync command will be ignored.
            "exec": ""
        },
        {
            "type": "gogs",
            "repo": "someuser/somerepo",
            "repo_url": "git@git.xxxx.com:someuser/somerepo.git",
            "secret": "!!EnterYourSecretHere!!",
            "store": "/path/to/store/the/repo",
            "target": "/path/to/deploy/target",
            "branch": "master"
        }
    ]
}
```

The deploy process
------------------------

1. Checkout or pull the latest `project.branch` into `project.store` directory (which will be create if not exist).
2. Run `project.post_checkout_script` in `project.store` directory.
3. Use `rsync` to synchronize files from `project.store` to `project.target`. (`.rsync_ignores` file will be used if it exists in `project.store` directory)
4. Run `project.post_rsync_script` in `project.target` directory.


Add webhook
------------

1. Set the `Payload URL` to `http://x.x.x.x:xx/github/pushed`
2. Set the `Content type` to `application/json`
3. Fill your `Secret`
4. Select `Just the push event`.
5. Finish and save.
6. The you can try a push.

# URLs:

- `/` => a simple 'Hello world' to test if the server is OK.
- `/github/pushed` => a handler for github
- `/gogs/pushed` => a handler for gogs.
- `/list-jobs` => a status list of all running jobs.


# FAQ

How to checkout files as another user, for example `www`, not `root`?
---------------------------------------------------------------------
You can use "run_as" field to specify which user to run the commands.




