package nsenter

/*
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>

// __attribute__((constructor)) 指的是，一旦这个包被引用，这个函数就会被自动执行
// 类似于构造函数，会在程序一启动的时候运行
__attribute__((constructor)) void enter_namespace(void) {
	char *mydocker_pid;
	// 从环境变量中获取需要进入PID
	mydocker_pid = getenv("cocin_docker_pid");
	if (mydocker_pid) {
		//fprintf(stdout, "got mydocker_pid=%s\n", mydocker_pid);
	} else {
		//fprintf(stdout, "missing mydocker_pid env skip nsenter");
		// 说明是父进程，直接退出
		return;
	}
	char *mydocker_cmd;
	// 从环境变量中获取需要执行的命令
	mydocker_cmd = getenv("cocin_docker_cmd");
	if (mydocker_cmd) {
		//fprintf(stdout, "got mydocker_cmd=%s\n", mydocker_cmd);
	} else {
		//fprintf(stdout, "missing mydocker_cmd env skip nsenter");
		// 没有指定命令，直接退出
		return;
	}
	int i;
	char nspath[1024];
	// 需要进入的5种Namespace
	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };

	for (i=0; i<5; i++) {
		// 拼接对应的路径 /proc/pid/ns/ipc 类似这样的
		sprintf(nspath, "/proc/%s/ns/%s", mydocker_pid, namespaces[i]);
		int fd = open(nspath, O_RDONLY);
		// 调用setns系统调用进入对应的Namespace  0代表：允许加入任何类型的 namespace
		if (setns(fd, 0) == -1) {
			//fprintf(stderr, "setns on %s namespace failed: %s\n", namespaces[i], strerror(errno));
		} else {
			//fprintf(stdout, "setns on %s namespace succeeded\n", namespaces[i]);
		}
		close(fd);
	}
	// 在进入的Namespace中执行指定的命令
	int res = system(mydocker_cmd);
	// 退出
	exit(0);
	return;
}
*/
import "C"
