targetScope = 'resourceGroup'

@description('Azure region for the private utility VM.')
param location string = 'southeastasia'

@description('Existing TinyURL virtual network name.')
param virtualNetworkName string = 'tinyurl-vnet'

@description('Name for the private utility VM.')
param vmName string = 'tinyurl-utility-vm'

@description('Private IPv4 address assigned to the utility VM.')
param privateIPAddress string = '10.20.4.4'

@description('Tiny VM size for private Redis and maintenance access.')
param vmSize string = 'Standard_B2ats_v2'

@description('Optional availability zone. Leave empty for regional placement.')
@allowed([
  ''
  '1'
  '2'
  '3'
])
param availabilityZone string = ''

@description('Admin username for maintenance access over Tailscale or private SSH.')
param adminUsername string = 'azureuser'

@secure()
@description('SSH public key for the utility VM admin user.')
param adminPublicKey string

var subnetName = 'utility'
var subnetPrefix = '10.20.4.0/24'
var nsgName = '${vmName}-nsg'
var nicName = '${vmName}-nic'
var osDiskName = '${vmName}-osdisk'

resource virtualNetwork 'Microsoft.Network/virtualNetworks@2023-11-01' existing = {
  name: virtualNetworkName
}

resource utilitySubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' = {
  name: subnetName
  parent: virtualNetwork
  properties: {
    addressPrefix: subnetPrefix
  }
}

resource networkSecurityGroup 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: nsgName
  location: location
  properties: {
    securityRules: [
      {
        name: 'AllowRedisFromVNet'
        properties: {
          priority: 100
          access: 'Allow'
          direction: 'Inbound'
          protocol: 'Tcp'
          sourcePortRange: '*'
          sourceAddressPrefix: 'VirtualNetwork'
          destinationPortRange: '6379'
          destinationAddressPrefix: '*'
        }
      }
      {
        name: 'AllowSSHFromVNet'
        properties: {
          priority: 110
          access: 'Allow'
          direction: 'Inbound'
          protocol: 'Tcp'
          sourcePortRange: '*'
          sourceAddressPrefix: 'VirtualNetwork'
          destinationPortRange: '22'
          destinationAddressPrefix: '*'
        }
      }
    ]
  }
}

resource networkInterface 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: nicName
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          privateIPAllocationMethod: 'Static'
          privateIPAddress: privateIPAddress
          subnet: {
            id: utilitySubnet.id
          }
        }
      }
    ]
    networkSecurityGroup: {
      id: networkSecurityGroup.id
    }
  }
}

resource utilityVM 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: vmName
  location: location
  zones: empty(availabilityZone) ? [] : [
    availabilityZone
  ]
  properties: {
    hardwareProfile: {
      vmSize: vmSize
    }
    osProfile: {
      computerName: vmName
      adminUsername: adminUsername
      linuxConfiguration: {
        disablePasswordAuthentication: true
        ssh: {
          publicKeys: [
            {
              path: '/home/${adminUsername}/.ssh/authorized_keys'
              keyData: adminPublicKey
            }
          ]
        }
      }
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: 'ubuntu-24_04-lts'
        sku: 'server'
        version: 'latest'
      }
      osDisk: {
        name: osDiskName
        createOption: 'FromImage'
        managedDisk: {
          storageAccountType: 'Standard_LRS'
        }
        diskSizeGB: 30
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: networkInterface.id
          properties: {
            deleteOption: 'Delete'
          }
        }
      ]
    }
    diagnosticsProfile: {
      bootDiagnostics: {
        enabled: false
      }
    }
  }
}

output vmName string = utilityVM.name
output privateIPAddress string = privateIPAddress
output redisURL string = 'redis://${privateIPAddress}:6379'
