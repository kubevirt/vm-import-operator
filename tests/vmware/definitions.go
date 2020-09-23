package vmware

// vcsim/0052-VirtualMachine-vm-70.xml
// vm-70 has one network and two disks
var (
	VM70 = "c39a8d6c-ea37-5c91-8979-334e7e07cab5"
	VM70Network = "VM Network"
	VM70MacAddress = "00:0c:29:5b:62:35"
	VM70DiskName1 = "disk-202-0"
	VM70DiskName2 = "disk-202-1"
	VM70Datastore = "/tmp/govcsim-DC0-LocalDS_0-024565671@folder-5"
)

// vcsim/0051-VirtualMachine-vm-66.xml
// vm-66 has no networks and one disk
var (
	VM66 = "f7c371d6-2003-5a48-9859-3bc9a8b08908"
	VM66DiskName = "disk-202-0"
	VM66Datastore = "/tmp/govcsim-DC0-LocalDS_0-024565671@folder-5"
)

// vcsim/0049-VirtualMachine-vm-63.xml
// vm-63 has no networks and one disk
var (
	VM63 = "cd0681bf-2f18-5c00-9b9b-8197c0095348"
)