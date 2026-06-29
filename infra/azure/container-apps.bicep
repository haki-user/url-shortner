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

@description('Short lowercase prefix used in resource names.')
param prefix string = 'tinyurl'

var environmentName = '${prefix}-environment'
var applicationName = '${prefix}-api'
var migrationJobName = '${prefix}-migrate'
var identityName = '${prefix}-container-identity'
var image = '${registry.properties.loginServer}/tinyurl-linkd:${imageTag}'
var publicURL = 'https://${applicationName}.${containerAppsEnvironment.properties.defaultDomain}'
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
              value: 'none'
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
output applicationURL string = 'https://${containerApp.properties.configuration.ingress.fqdn}'
output environmentName string = containerAppsEnvironment.name
output migrationJobName string = migrationJob.name
output managedIdentityName string = containerIdentity.name
