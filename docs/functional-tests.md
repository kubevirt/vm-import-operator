# Functional tests

[Functional tests](/tests) for the vm-import-operator are listed below.

## Basic VM
| Test description | Implemented |
| :---------------- | :-----------: |
| Basic VM import without resource mapping should create stopped VM | &check; |
| Basic import without resource mapping should create started VM | &check; |
| Basic import with in-CR resource mapping should create running VM for storage domain mapping | &check; |
| Basic import with in-CR resource mapping should create running VM for storage disk mapping | &check; |

## Basic Net VM
| Test description | Implemented |
| :---------------- | :-----------: |
| Networked VM import should create started VM | &check; |

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
