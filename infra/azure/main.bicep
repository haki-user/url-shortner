targetScope = 'resourceGroup'

@description('Azure region for all resources.')
param location string = resourceGroup().location

@description('Short lowercase prefix used in resource names.')
@minLength(3)
@maxLength(12)
param prefix string = 'tinyurl'

@description('PostgreSQL administrator username.')
param postgresAdminUsername string = 'tinyurladmin'

@secure()
@description('PostgreSQL administrator password. Use URL-safe characters.')
param postgresAdminPassword string

var suffix = uniqueString(subscription().id, resourceGroup().id)
var compactPrefix = replace(toLower(prefix), '-', '')
var networkName = '${prefix}-vnet'
var registryName = take('${compactPrefix}${suffix}', 50)
var postgresServerName = take('${compactPrefix}-pg-${suffix}', 63)
var keyVaultName = take('${compactPrefix}-kv-${suffix}', 24)
var postgresPrivateDnsZoneName = '${postgresServerName}.private.postgres.database.azure.com'

resource virtualNetwork 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: networkName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        '10.20.0.0/16'
      ]
    }
    subnets: [
      {
        name: 'postgres'
        properties: {
          addressPrefix: '10.20.2.0/24'
          delegations: [
            {
              name: 'postgres-flexible-server'
              properties: {
                serviceName: 'Microsoft.DBforPostgreSQL/flexibleServers'
              }
            }
          ]
        }
      }
      {
        name: 'container-apps'
        properties: {
          addressPrefix: '10.20.3.0/27'
          delegations: [
            {
              name: 'container-apps'
              properties: {
                serviceName: 'Microsoft.App/environments'
              }
            }
          ]
        }
      }
    ]
  }
}

resource postgresSubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' existing = {
  name: 'postgres'
  parent: virtualNetwork
}

resource postgresPrivateDnsZone 'Microsoft.Network/privateDnsZones@2020-06-01' = {
  name: postgresPrivateDnsZoneName
  location: 'global'
}

resource postgresPrivateDnsLink 'Microsoft.Network/privateDnsZones/virtualNetworkLinks@2020-06-01' = {
  name: '${prefix}-postgres-link'
  parent: postgresPrivateDnsZone
  location: 'global'
  properties: {
    registrationEnabled: false
    virtualNetwork: {
      id: virtualNetwork.id
    }
  }
}

resource postgres 'Microsoft.DBforPostgreSQL/flexibleServers@2024-08-01' = {
  name: postgresServerName
  location: location
  sku: {
    name: 'Standard_B1ms'
    tier: 'Burstable'
  }
  properties: {
    administratorLogin: postgresAdminUsername
    administratorLoginPassword: postgresAdminPassword
    version: '16'
    storage: {
      storageSizeGB: 32
    }
    backup: {
      backupRetentionDays: 7
      geoRedundantBackup: 'Disabled'
    }
    highAvailability: {
      mode: 'Disabled'
    }
    network: {
      delegatedSubnetResourceId: postgresSubnet.id
      privateDnsZoneArmResourceId: postgresPrivateDnsZone.id
    }
  }
  dependsOn: [
    postgresPrivateDnsLink
  ]
}

resource database 'Microsoft.DBforPostgreSQL/flexibleServers/databases@2024-08-01' = {
  name: 'tinyurl'
  parent: postgres
  properties: {}
}

resource registry 'Microsoft.ContainerRegistry/registries@2023-07-01' = {
  name: registryName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    adminUserEnabled: false
    publicNetworkAccess: 'Enabled'
  }
}

resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' = {
  name: keyVaultName
  location: location
  properties: {
    tenantId: subscription().tenantId
    enableRbacAuthorization: true
    enableSoftDelete: true
    softDeleteRetentionInDays: 7
    sku: {
      family: 'A'
      name: 'standard'
    }
  }
}

resource databaseURL 'Microsoft.KeyVault/vaults/secrets@2023-07-01' = {
  name: 'tinyurl-database-url'
  parent: keyVault
  properties: {
    value: 'postgres://${postgresAdminUsername}:${postgresAdminPassword}@${postgres.properties.fullyQualifiedDomainName}:5432/tinyurl?sslmode=require'
  }
}

output registryName string = registry.name
output registryLoginServer string = registry.properties.loginServer
output keyVaultName string = keyVault.name
output postgresServerName string = postgres.name
output postgresHost string = postgres.properties.fullyQualifiedDomainName
