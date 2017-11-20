############### Check Installed Packages ############################

describe package('tftpd-hpa') do
  it { should be_installed }
end

describe package('apache2') do
  it { should be_installed }
end

describe port(80) do
    it { should be_listening }
  end

describe port(68) do
    it { should be_listening }
  end
  
describe port(69) do
    it { should be_listening }
  end

describe port(19123) do
    it { should be_listening }
  end

# extend tests with metadata
control '01' do
  impact 0.7
  title 'Verify mcp2-dhcp-server'
  desc 'Ensures mcp2-dhcp-server is up and running'
  describe service('mcp2-dhcp-server') do
    it { should be_enabled }
    it { should be_installed }
    #it { should be_running }
    #In test environment the service will never start because it will never be on the same vlan
  end
end

# extend tests with metadata
control '02' do
  impact 0.7
  title 'Verify cloud-config-server'
  desc 'Ensures cloud-config-server is up and running'
  describe service('cloud-config-server') do
    it { should be_enabled }
    it { should be_installed }
    it { should be_running }
  end
end


