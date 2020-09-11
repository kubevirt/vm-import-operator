package vmware

// vcsim/0052-VirtualMachine-vm-70.xml
// vm-70 has one network and two disks
var (
	VM70 = "c39a8d6c-ea37-5c91-8979-334e7e07cab5"
	VM70Network = "VM Network"
	VM70MacAddress = "00:0c:29:5b:62:35"
)

// vcsim/0051-VirtualMachine-vm-66.xml
// vm-66 has no networks and one disk
var (
	VM66 = "f7c371d6-2003-5a48-9859-3bc9a8b08908"
	VM66DiskName = "disk-202-0"
	VM66Datastore = "/tmp/govcsim-DC0-LocalDS_0-024565671@folder-5"
)
