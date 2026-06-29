targetScope = 'resourceGroup'

@description('Azure region for Container Apps resources.')
param location string = 'southeastasia'

@description('Existing TinyURL virtual network name.')
param virtualNetworkName string = 'tinyurl-vnet'

@description('Existing Azure Container Registry name.')
param registryName string

@description('Existing Key Vault name.')
param keyVaultName string

@description('Immutable application image tag in the existing registry.')
param imageTag string

@description('Public HTTPS origin returned in created short links.')
param customDomain string = 'tinyurl.haki-user.in'

@description('Short lowercase prefix used in resource names.')
param prefix string = 'tinyurl'

var environmentName = '${prefix}-environment'
var applicationName = '${prefix}-api'
var redisName = '${prefix}-redis'
var migrationJobName = '${prefix}-migrate'
var identityName = '${prefix}-container-identity'
var image = '${registry.properties.loginServer}/tinyurl-linkd:${imageTag}'
var redisImage = '${registry.properties.loginServer}/tinyurl-redis:7.4-alpine'
var publicURL = 'https://${customDomain}'
var databaseSecretURL = 'https://${keyVault.name}${environment().suffixes.keyvaultDns}/secrets/tinyurl-database-url'
var acrPullRoleDefinitionId = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  '7f951dda-4ed3-4680-a7ca-43fe172d538d'
)
var keyVaultSecretsUserRoleDefinitionId = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  '4633458b-17de-408a-b874-0445c86b69e6'
)

resource virtualNetwork 'Microsoft.Network/virtualNetworks@2023-11-01' existing = {
  name: virtualNetworkName
}

resource containerAppsSubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' = {
  name: 'container-apps'
  parent: virtualNetwork
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

resource registry 'Microsoft.ContainerRegistry/registries@2023-07-01' existing = {
  name: registryName
}

resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' existing = {
  name: keyVaultName
}

resource containerIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: identityName
  location: location
}

resource acrPullAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(registry.id, containerIdentity.id, acrPullRoleDefinitionId)
  scope: registry
  properties: {
    roleDefinitionId: acrPullRoleDefinitionId
    principalId: containerIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource keyVaultAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(keyVault.id, containerIdentity.id, keyVaultSecretsUserRoleDefinitionId)
  scope: keyVault
  properties: {
    roleDefinitionId: keyVaultSecretsUserRoleDefinitionId
    principalId: containerIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource containerAppsEnvironment 'Microsoft.App/managedEnvironments@2024-03-01' = {
  name: environmentName
  location: location
  properties: {
    vnetConfiguration: {
      infrastructureSubnetId: containerAppsSubnet.id
      internal: false
    }
    workloadProfiles: [
      {
        name: 'Consumption'
        workloadProfileType: 'Consumption'
      }
    ]
  }
}

resource redisApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: redisName
  location: location
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${containerIdentity.id}': {}
    }
  }
  properties: {
    managedEnvironmentId: containerAppsEnvironment.id
    workloadProfileName: 'Consumption'
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: false
        targetPort: 6379
        exposedPort: 6379
        transport: 'tcp'
      }
      registries: [
        {
          server: registry.properties.loginServer
          identity: containerIdentity.id
        }
      ]
    }
    template: {
      containers: [
        {
          name: 'redis'
          image: redisImage
          command: [
            'redis-server'
          ]
          args: [
            '--save'
            ''
            '--appendonly'
            'no'
            '--bind'
            '0.0.0.0'
            '--protected-mode'
            'no'
            '--maxmemory'
            '128mb'
            '--maxmemory-policy'
            'allkeys-lru'
          ]
          resources: {
            cpu: json('0.25')
            memory: '0.5Gi'
          }
          probes: [
            {
              type: 'Liveness'
              tcpSocket: {
                port: 6379
              }
              initialDelaySeconds: 3
              periodSeconds: 30
              timeoutSeconds: 2
              failureThreshold: 3
            }
            {
              type: 'Readiness'
              tcpSocket: {
                port: 6379
              }
              initialDelaySeconds: 1
              periodSeconds: 10
              timeoutSeconds: 2
              failureThreshold: 3
            }
          ]
        }
      ]
      scale: {
        minReplicas: 1
        maxReplicas: 1
      }
    }
  }
  dependsOn: [
    acrPullAssignment
  ]
}

resource migrationJob 'Microsoft.App/jobs@2024-03-01' = {
  name: migrationJobName
  location: location
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${containerIdentity.id}': {}
    }
  }
  properties: {
    environmentId: containerAppsEnvironment.id
    workloadProfileName: 'Consumption'
    configuration: {
      triggerType: 'Manual'
      replicaTimeout: 600
      replicaRetryLimit: 1
      manualTriggerConfig: {
        parallelism: 1
        replicaCompletionCount: 1
      }
      registries: [
        {
          server: registry.properties.loginServer
          identity: containerIdentity.id
        }
      ]
      secrets: [
        {
          name: 'database-url'
          keyVaultUrl: databaseSecretURL
          identity: containerIdentity.id
        }
      ]
    }
    template: {
      containers: [
        {
          name: 'migrate'
          image: image
          command: [
            '/app/migrate'
          ]
          env: [
            {
              name: 'TINYURL_DATABASE_URL'
              secretRef: 'database-url'
            }
          ]
          resources: {
            cpu: json('0.25')
            memory: '0.5Gi'
          }
        }
      ]
    }
  }
  dependsOn: [
    acrPullAssignment
    keyVaultAssignment
  ]
}

resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: applicationName
  location: location
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${containerIdentity.id}': {}
    }
  }
  properties: {
    managedEnvironmentId: containerAppsEnvironment.id
    workloadProfileName: 'Consumption'
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: true
        allowInsecure: false
        targetPort: 8080
        transport: 'auto'
      }
      registries: [
        {
          server: registry.properties.loginServer
          identity: containerIdentity.id
        }
      ]
      secrets: [
        {
          name: 'database-url'
          keyVaultUrl: databaseSecretURL
          identity: containerIdentity.id
        }
      ]
    }
    template: {
      containers: [
        {
          name: 'linkd'
          image: image
          env: [
            {
              name: 'TINYURL_STORAGE'
              value: 'postgres'
            }
            {
              name: 'TINYURL_DATABASE_URL'
              secretRef: 'database-url'
            }
            {
              name: 'TINYURL_CACHE'
              value: 'redis'
            }
            {
              name: 'TINYURL_REDIS_URL'
              value: 'redis://${redisApp.properties.configuration.ingress.fqdn}:6379'
            }
            {
              name: 'TINYURL_ADDR'
              value: ':8080'
            }
            {
              name: 'TINYURL_BASE_URL'
              value: publicURL
            }
          ]
          resources: {
            cpu: json('0.25')
            memory: '0.5Gi'
          }
          probes: [
            {
              type: 'Liveness'
              httpGet: {
                path: '/healthz'
                port: 8080
                scheme: 'HTTP'
              }
              initialDelaySeconds: 5
              periodSeconds: 30
              timeoutSeconds: 2
              failureThreshold: 3
            }
            {
              type: 'Readiness'
              httpGet: {
                path: '/readyz'
                port: 8080
                scheme: 'HTTP'
              }
              initialDelaySeconds: 3
              periodSeconds: 10
              timeoutSeconds: 2
              failureThreshold: 3
            }
          ]
        }
      ]
      scale: {
        minReplicas: 0
        maxReplicas: 3
        rules: [
          {
            name: 'http'
            http: {
              metadata: {
                concurrentRequests: '50'
              }
            }
          }
        ]
      }
    }
  }
  dependsOn: [
    acrPullAssignment
    keyVaultAssignment
  ]
}

output applicationName string = containerApp.name
output applicationURL string = publicURL
output platformURL string = 'https://${containerApp.properties.configuration.ingress.fqdn}'
output environmentName string = containerAppsEnvironment.name
output migrationJobName string = migrationJob.name
output managedIdentityName string = containerIdentity.name
output redisName string = redisApp.name
