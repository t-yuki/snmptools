#include <inttypes.h>
#include <stdlib.h>
#include <net-snmp/net-snmp-config.h>
#include <net-snmp/net-snmp-includes.h>
#include <net-snmp/agent/net-snmp-agent-includes.h>

void sitemon_init_agent(char *sockaddr);
void sitemon_run_agent(void);
void sitemon_stop_agent(void);
int sitemon_agent_running(void);

void sitemon_register_scalar(char *name, ulong *sitemon_oid, int oid_length);
