# oVirt to Kubevirt rules to import virtual machine

## Actions

Each validation failure will result in a predefined action being performed.

The description of the actions is following:
* **Log** - rule validation failure will result in logging that information to operator logs and the CR will be processed;
* **Warn** - rule validation failure will result in saving that information in the CR and the CR will be processed (import will be executed);
* **Block** - rule validation failure will result in savint that information in the CR and the CR will not be processed (import will not be executed).

## Networking rules

### Preconditions

Network to be used by the imported virtual machine has been properly pre-configured in the target cluster by its administrator (host network configuration and network attachment definition).

### Rules

ID | Predicate | Action
--- | --- | ---
1 | Source network can’t be mapped to exactly one target network. Each network’s name linked to VM’s Nics (VM.nics[].name) has corresponding network with the same name in target cluster OR mapping between these names has been provided in the migration manifest. | Block
2 | VM.nics[].interface not in [e1000, rtl8139, virtio] | Block
3 | Vm.nics[].on_boot == false | Log
4 | Vm.nics[].plugged == false | Warn
5 | VM.nics[].vnic_profile.port_mirroring == true | Warn
6 | VM.nics[].vnic_profile.custom_properties | Warn
7 | VM.nics[].vnic_profile.network_filter | Warn
8 | VM.nics[].vnic_profile.qos | Log

## Storage rules

### Rules

ID | Predicate | Action
--- | --- | ---
1 | Vm.disk_attachments[].disk.interface (deprecated) or Vm.disk_attachments[].interface is other than [sata, virtio_scsi, virtio] | Block
2 | Vm.disk_attachments[].disk.backup == incremental | Warn
3 | Vm.disk_attachments[].disk.lun_storage set | Block
4 | VM.disk_attachments[].disk.propagate_errors | Log
5 | VM.disk_attachments[].disk.wipe_after_delete == true | Log
6 | VM.disk_attachments[].disk.status == [illegal, locked] | Block
7 | VM.disk_attachments[].disk.storage_type != image | Block
8 | VM.disk_attachments[].disk.logical_name is set | Log
9 | VM.disk_attachments[].disk.uses_scsi_reservation == true | Block
10 | VM.disk_attachments[].disk.Sgio is set | Block
11 | VM.disk_attachments[].logical_name is set | Log
12 | VM.disk_attachments[].pass_discard == true | Log
13 | VM.disk_attachments[].uses_scsi_reservation == true | Block
14 | VM.disk_attachments is empty | Block

## VM configuration rules

### Preconditions

1. If VM requires GPU resources, nodes providing GPUs need to be configured by the administrator before the migration. vGPU - block

### Rules

ID | Predicate | Action
--- | --- | ---
1 | VM.bios.boot_menu.enabled == true | Log
2 | VM.bios.type not matching ClusterConfig.EmulatedMachines | Block
3 | VM.bios.type == q35_secure_boot | Warn
4 | VM.cpu.architecture == s390x (default) or anything that does not match ClusterConfig.EmulatedMachines | Block
5 | VM.cpu.cpu_tune setting is other than 1 vCPU- 1 pCPU. Allowed config example: ```<cputune><vcpupin vcpu="0" cpuset="0"/><vcpupin vcpu="1" cpuset="1"/><vcpupin vcpu="2" cpuset="2"/><vcpupin vcpu="3" cpuset="3"/></cputune>``` | Warn
6 | VM.cpu_shares set | Log
7 | VM.custom_properties set | Warn
8 | VM.display.type == spice | Log
9 | VM.has_illegal_images == true | Block
10 | VM.high_availability.priority set | Log
11 | VM.io.threads set | Warn
12 | VM.memory_policy.ballooning == true | Log
13 | VM.memory_policy.over_commit.percent set | Log
14 | VM.memory_policy.guaranteed set | Log
15 | VM.migration set | Log
16 | VM.migration_downtime set | Log
17 | VM.numa_tune_mode set | Warn
18 | VM.origin == kubevirt | Block
19 | VM.Rng_device.source other than urandom | Log
20 | VM.soundcard_enabled == true | Warn
21 | VM.start_paused == true | Log
22 | VM.storage_error_resume_behaviour set | Log
23 | VM.tunnel_migration | Warn
24 | VM.usb.enabled == true | Block
25 | VM->graphics_consoles[].protocol == spice | Log
26 | VM->host_devices | Log
27 | VM->reported_devices | Log
28 | VM->quota has been configured for the VM | Log
29 | VM->watchdogs[].model == diag288 | Block
30 | VM.cdroms[].file.storage_domain.type != data | Log
31 | VM.floppies[] is not empty | Log
32 | vm.timezone is not UTC-compatible | Block
33 | vm.status other than 'up' or 'down' | Block