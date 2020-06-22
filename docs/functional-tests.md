# Functional tests

[Functional tests](/tests) for the vm-import-operator are listed below.

## Basic VM
| Test description | Implemented |
| :---------------- | :-----------: |
| Basic VM import without resource mapping should create stopped VM | &check; |
| Basic import without resource mapping should create started VM | &check; |
| Basic import with in-CR resource mapping should create running VM for storage domain mapping | &check; |
| Basic import with in-CR resource mapping should create running VM for storage disk mapping | &check; |
| Target VM and VMI should not be affected when successful import CR is removed | &check; |

## Basic VM negative
| Test description | Implemented |
| :---------------- | :-----------: |
| Basic VM import with invalid VM image should fail import | &check; |
| Basic VM import with invalid VM image size should fail import | &check; |
| Basic VM import with missing oVirt secret should fail import | &check; |
| Basic VM import with invalid oVirt secret should fail import | &check; |
| Basic VM import with invalid oVirt cert should fail import | &check; |
| Basic VM import with invalid oVirt URL should fail import | &check; |
| Basic VM import with invalid oVirt username should fail import | &check; |
| Basic VM import with invalid oVirt password should fail import | &check; |
| Basic VM import with non-existing VM ID should fail import | &check; |
| Basic VM import with missing specified external resource mapping should fail import | &check; |
| Basic VM import should be stuck retrying to stop the source VM when it does not shut down | &check; |

## Basic Net VM
| Test description | Implemented |
| :---------------- | :-----------: |
| Networked VM import should create started VM | &check; |

### Multiple VMs import
| Test description | Implemented |
| :---------------- | :-----------: |
| Two VMs should be imported in parallel to the same namespace and started | &cross; |
| Two VMs should be imported in sequence to the same namespace and started | &check; |
| Import of the same source VM with NIC (and MAC address) to the same namespace should fail | &check; |
| Import of the same source VM with NIC (and MAC address) to a different namespace should fail | &check; |
| Same source VM with no NICs imported second time with the same target name to the same namespace should not be created but import should be successful | &check; |
| Same VM with no NICs imported second time in a row to a different namespace than the first time should be started | &check; |

## Various VM configurations
| Test description | Implemented |
| :---------------- | :-----------: |
| VM with 'up' status should be imported and started | &check; |
| VM with 'q35_secure_boot' BIOS type should be imported and started | &check; |
| VM with 'q35_ovmf' BIOS type should be imported and started | &check; |
| VM with 'ppc64' architecture should be imported and started | &cross; |
| VM with 'undefined' architecture should be imported and started | &check; |
| VM with 'ovirt' origin should be imported and started | &check; |
| VM with placement policy affinity set to 'user_migratable' should be imported and started | &check; |
| VM with placement policy affinity set to 'migrateble' and Live Migration enabled should be imported and started | pending fix to #267 |
| VM with placement policy affinity set to 'pinned' should be imported and started | &check; |
| VM with UTC-compatible (i.e. 'Africa/Abidjan')  timezone should be imported and started | &check; |
| VM with correct pinning layout should be imported and started | &check; |
| VM with disabled USB should be imported and started | &check; |
| VM with 'i6300esb' watchdog should be imported and started | &check; |

## Cancel VM Import
| Test description | Implemented |
| :---------------- | :-----------: |
| VM import cancellation should have deleted all the import-associated resources when VM Import is deleted in the foreground | &check; |

## VM Network Validation
| Test description | Implemented |
| :---------------- | :-----------: |
| VM with unsupported NIC interfaces should be blocked | &check; |
| VM with vnic profile pass-through enabled should be blocked | &check; |

## VM Storage Validation
| Test description | Implemented |
| :---------------- | :-----------: |
| VM with no disk attachments should be blocked | &check; |
| VM with unsupported disk attachment interface should be blocked | &check; |
| VM with disk attachment with SCSI reservation should be blocked | &check; |
| VM with unsupported disk interface should be blocked | &check; |
| VM with disk with SCSI reservation should be blocked | &check; |
| VM with disk with LUN storage should be blocked | &check; |
| VM with disk with status other than 'ok' should be blocked | &check; |
| VM with disk with storage other than 'image' should be blocked | &check; |
| VM with disk with SGIO set to "filtered" should be blocked | &check; |
| VM with disk with SGIO set to "unfiltered" should be blocked | &check; |

## VM Validation
| Test description | Implemented |
| :---------------- | :---------: |
| VM with status other than 'up' or 'down' should be blocked | &check; |
| VM with unsupported BIOS type should be blocked | &check; |
| VM with unsupported CPU architecture should be blocked | &check; |
| VM with illegal images should be blocked | &check; |
| VM with 'kubevirt' origin should be blocked | &check; |
| VM with placement policy affinity set to 'migratable' should be blocked | &check; |
| VM with USB enabled should be blocked | &check; |
| VM with watchdog other than 'i6300esb' should be blocked | &check; |
| VM with non-UTC-compatible (i.e. 'America/New_York')  timezone should be blocked | &check; |

## Resource mapping validation
| Test description | Implemented |
| :---------------- | :---------:
| Import with missing network resource mapping should be blocked | &check; |
| Import with network mapping to a non-existing target network should be blocked | &check; |
| Import with storage mapping to a non-existing target storage class should be blocked | &check; |
| Import with disk mapping to a non-existing target storage class should be blocked | &check; |

## Resource mapping
| Test description | Implemented |
| :---------------- | :---------: |
| Import with external resource mapping for network should create running VM | &check; |
| Import with external resource mapping for disk should create running VM with default storage class ignoring external mapping | &check; |
| Import with external resource mapping for storage domain should create running VM | &check; |
| Import with external resource mapping for storage and in-CR for network should create running VM | &check; |
| Import with in-CR resource mapping overriding external resource mapping for network should create running VM | &check; |
| Import with in-CR resource mapping overriding external resource mapping for storage domain should create running VM | &check; |

## Multiple disks
| Test description | Implemented |
| :---------------- | :---------: |
| Import of a VM with two disks should create running VM and set correct boot order | &check; |

## Networking
| Test description | Implemented |
| :---------------- | :---------: |
| Import of VM should create running VM with Multus network | &check; |
| Import of VM should create running VM with two networks: Multus and Pod | &check; |
| Import of VM should create running VM with two Multus networks | &check; |

## Templates
| Test description | Implemented |
| :---------------- | :---------: |
| Import of a Windows VM should detect and apply correct template to the target VM | &cross; |
| Import of a Linux VM should detect and apply correct template to the target VM | &cross; |
