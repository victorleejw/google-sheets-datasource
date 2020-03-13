import { DataSourceInstanceSettings, SelectableValue } from '@grafana/data';
import { DataSourceWithBackend, getBackendSrv } from '@grafana/runtime';

import { SheetsQuery, SheetsSourceOptions } from './types';


export enum HealthStatus {
  UNKNOWN = 'UNKNOWN',
  OK = 'OK',
  ERROR = 'ERROR',
}

export interface HealthCheckResult {
  status: HealthStatus;
  message: string;
  details?: Record<string,any>;
}

export class DataSource extends DataSourceWithBackend<SheetsQuery, SheetsSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<SheetsSourceOptions>) {
    super(instanceSettings);
  }

  async getSpreadSheets(): Promise<Array<SelectableValue<string>>> {
    return this.getResource('spreadsheets').then(({ spreadsheets }) =>
      spreadsheets ? Object.entries(spreadsheets).map(([value, label]) => ({ label, value } as SelectableValue<string>)) : []
    );
  }

  /**
   * Run the datasource healthcheck
   */
  async callHealthCheck(): Promise<HealthCheckResult> {
    // TODO: if the service is ERROR it returns 503... this causes a popup
    return getBackendSrv().get(`/api/datasources/${this.id}/health`)
  }

  /**
   * Checks the plugin health
   */
  async testDatasource(): Promise<any> {
    return this.callHealthCheck().then( res => {
      console.log( 'TEST', res );
      if(res.status === HealthStatus.OK) {
        return {
          status: 'success',
          message: res.message,
        };
      }
      return {
          status: 'fail',
          message: res.message,
      }
    });
  }
}
