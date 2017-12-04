Vagrant.configure('2') do |config|
    
        config.vm.provider :vmware_fusion do |vm|
            
            vdiskmanager = '/Applications/VMware\ Fusion.app/Contents/Library/vmware-vdiskmanager'
    
            dir = "#{ENV['HOME']}/vagrant-additional-disk"
    
            unless File.directory?( dir )
                Dir.mkdir dir
            end
    
            file_to_disk = "#{dir}/var-lib-mysql.vmdk"
    
            unless File.exists?( file_to_disk )
                `#{vdiskmanager} -c -s 20GB -a lsilogic -t 1 #{file_to_disk}`
            end
    
            vm.vmx['scsi0:1.filename'] = file_to_disk
            vm.vmx['scsi0:1.present']  = 'TRUE'
            vm.vmx['scsi0:1.redo']     = ''
    
        end
end