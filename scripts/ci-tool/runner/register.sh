#!/bin/bash
set -e

 #./register-runner.sh http://192.168.10.162 egame-tool-runner glrt-YysPNp07gYj44UQTqSz_p286MQpwOjEKdDozCnU6NA8.01.171945ko0 /data/runner-compose/runner-egame-tool "egame-tool-runner"


 ./register-runner.sh http://192.168.10.162 egame-runner glrt-gcs5veeJSb5yWOEVNlFAdG86MQpwOjEKdDozCnU6NA8.01.1704zazf8 /data/runner-compose/runner-egame "egame-runner"
 sleep 2

./register-runner.sh http://192.168.10.162 egame-tool-runner glrt-YysPNp07gYj44UQTqSz_p286MQpwOjEKdDozCnU6NA8.01.171945ko0 /data/runner-compose/runner-egame-tool "egame-tool-runner"
 sleep 2

./register-runner.sh http://192.168.10.162 grpc03-runner glrt-flQbcFuOpoJVQTzi6h8t2W86MQpwOjMKdDozCnU6NA8.01.1705wo7c7 /data/runner-compose/runner-grpc03 "grpc03-runner"
sleep 2

./register-runner.sh http://192.168.10.162 logic03-runner glrt-ZTY9XP0rHwPVNOrpkP2iRW86MQpwOjQKdDozCnU6NA8.01.171d3j6t9 /data/runner-compose/runner-logic03 "logic03-runner"
sleep 2


#./register-runner.sh http://192.168.10.162 egame-runner glrt-gcs5veeJSb5yWOEVNlFAdG86MQpwOjEKdDozCnU6NA8.01.1704zazf8 /data/runner-compose/runner-egame "egame-runner"
#sleep 5


