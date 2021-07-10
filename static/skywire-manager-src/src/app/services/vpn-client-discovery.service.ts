import { Injectable } from '@angular/core';
import { Observable, of } from 'rxjs';
import { HttpClient } from '@angular/common/http';
import { retryWhen, delay, map } from 'rxjs/operators';

/**
 * Ratings some properties of a server can have.
 *
 * TODO: used only for columns that are currently deactivated in the server list. Must
 * be deleted or reactivated depending on what happens to the columns.
 */
/*
export enum Ratings {
  Gold = 0,
  Silver = 1,
  Bronze = 2,
}
*/

/**
 * Data of a server obtained from the discovery service.
 */
export class VpnServer {
  /**
   * 2 letter code of the country the server is in.
   */
  countryCode: string;
  /**
   * Sever name.
   */
  name: string;
  /**
   * Location of the server.
   */
  location: string;
  /**
   * Public key.
   */
  pk: string;


  // TODO: used only for columns that are currently deactivated in the server list. Must
  // be deleted or reactivated depending on what happens to the columns.
  /**
   * Current congestion of the server.
   */
   // congestion: number;
  /**
   * Rating of the congestion the server normally has.
   */
  // congestionRating: Ratings;
  /**
   * Latency of the server.
   */
  // latency: number;
  /**
   * Rating of the latency the server normally has.
   */
  // latencyRating: Ratings;
  /**
   * Hops needed for reaching the server.
   */
  // hops: number;


  /**
   * Note the server has in the discovery service.
   */
  note: string;
}

/**
 * Allows to get the vpn servers registered in the discovery service. The service was made for
 * the VPN client.
 *
 * IMPORTANT: changes in the discovery service are needed before being able to get all the data.
 */
@Injectable({
  providedIn: 'root'
})
export class VpnClientDiscoveryService {
  /**
   * URL of the discovery service.
   */
  private readonly discoveryServiceUrl = 'https://service.discovery.skycoin.com/api/services?type=vpn';

  /**
   * Servers obtained from the discovery service.
   */
  private servers: VpnServer[];

  constructor(
    private http: HttpClient,
  ) {}

  /**
   * Gets the vpn servers registered in the discovery service. After the first call, it
   * will return the same list in all future calls, to avoid having to make more network
   * connections, until the app is reloaded.
   */
  getServers(): Observable<VpnServer[]> {
    // If a server list was obtained before, return it.
    if (this.servers) {
      return of(this.servers);
    }

    return this.http.get(this.discoveryServiceUrl).pipe(
      // In case of error, retry.
      retryWhen(errors => errors.pipe(delay(4000))),
      map((result: any[]) => {
        const response: VpnServer[] = [];

        // Process the data.
        result.forEach(entry => {
          const currentEntry = new VpnServer();

          // The address must have 2 parts: the pk and the port.
          const addressParts = (entry.address as string).split(':');
          if (addressParts.length === 2) {
            currentEntry.pk = addressParts[0];

            // Process the location.
            currentEntry.location = '';
            if (entry.geo) {
              if (entry.geo.country) {
                currentEntry.countryCode = entry.geo.country;
              }
              if (entry.geo.region) {
                currentEntry.location = entry.geo.region;
              }
            }

            currentEntry.name = addressParts[0];
            /*
            // TODO: used only for columns that are currently deactivated in the server list. Must
            // be deleted or reactivated depending on what happens to the columns.
            currentEntry.congestion = 20;
            currentEntry.congestionRating = Ratings.Gold;
            currentEntry.latency = 123;
            currentEntry.latencyRating = Ratings.Gold;
            currentEntry.hops = 3;
            */

            // TODO: still not added to the discovery service.
            currentEntry.note = '';

            response.push(currentEntry);
          }
        });

        this.servers = response;

        return response;
      })
    );
  }
}
