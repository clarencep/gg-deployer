#include <sys/types.h>
#include <pwd.h>
#include <grp.h>
#include <stdlib.h>
#include <unistd.h>

int id_user(const char* user) {
    struct passwd *pwd = NULL;

    pwd = getpwnam(user);
    if (pwd == NULL) {
        return -1;
    }

    return pwd->pw_uid;
}


int id_group(const char* group) {
    struct group *grp = NULL;

    grp = getgrnam(group);
    if (grp == NULL) {
        return -1;
    }

    return grp->gr_gid;
}


