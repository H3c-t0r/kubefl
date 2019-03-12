import '@polymer/iron-flex-layout/iron-flex-layout-classes.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/iron-icon/iron-icon.js';
import '@polymer/iron-icons/iron-icons.js';
import '@polymer/paper-progress/paper-progress.js';

import {html, PolymerElement} from '@polymer/polymer';
import './activities-list.js';

export class ActivityView extends PolymerElement {
    static get template() {
        return html`
            <style is="custom-style" include="iron-flex iron-flex-alignment">
            </style>
            <style>
                :host {
                    @apply --layout-fit;
                    --accent-color: #007dfc;
                    --primary-background-color: #003c75;
                    --sidebar-default-color: #ffffff4f;
                    --border-color: #f4f4f6;
                    background: #f1f3f4;
                    min-height: 50%;
                    padding: 1em;
                }
                paper-progress {
                    width: 100%;
                    --paper-progress-active-color: var(--accent-color)
                }
                p {
                    background: #fff;
                    border-top: 1px solid rgba(0,0,0,.12);
                    box-shadow: 0 3px 3px rgba(0,0,0,.12);
                    transition: margin .2s cubic-bezier(0.4, 0, 0.2, 1);
                    font-size: 13px;
                    margin: 0px;
                    padding: 5px;
                }
                [hidden] {
                    display: none;
                    opacity: 0;
                    pointer-events: none
                }
            </style>
            <iron-ajax id="ajax" url="/api/activities/[[namespace]]"
                handle-as="json" loading="{{loading}}"
                on-response="_onResponse">
            </iron-ajax>
            <paper-progress indeterminate class="slow"
                hidden$="[[!loading]]"></paper-progress>
            <p hidden$="[[!message]]">[[message]]</p>
            <activities-list activities="[[activities]]"></activities-list>
            `;
    }

    /**
     * Object describing property-related metadata used by Polymer features
     */
    static get properties() {
        return {
            namespace: {
                type: String,
                observer: '_namespaceChanged',
                value: null,
            },
            message: {
                type: String,
                value: 'Select a namespace to see recent events',
            },
            loading: {
                type: Boolean,
                value: false,
            },
            activities: {
                type: Array,
                value: [],
            },
        };
    }

    /**
     * Retrieves Events when namespace is selected.
     * @param {string} newNamespace
     */
    _namespaceChanged(newNamespace) {
        if (newNamespace) {
            this.message = '';
            this.$['ajax'].generateRequest();
        }
    }

    /**
     * Handles the Activities response to set date format and icon.
     * @param {Event} responseEvent
     */
    _onResponse(responseEvent) {
        const {status, response} = responseEvent.detail;
        this.splice('activities', 0);
        if (status !== 200) {
            this.message =
                `Error retrieving activities for namespace ${this.namespace}`;
        } else if (!response.length) {
            this.message = `No activities for namespace ${this.namespace}`;
        } else {
            this.push('activities', ...response);
        }
    }
}

window.customElements.define('activity-view', ActivityView);
