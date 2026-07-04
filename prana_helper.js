/**
 * prana_helper.js
 *
 * Helper JS mínimo para interop com o wings Go/WASM.
 * Deve ser carregado ANTES do wasm_exec.js e do binário .wasm.
 *
 * Define window._pranaDef(tagName, ctor, connected, attrChanged, disconnected, observed,
 * formAssociated) que é chamado pelo Go via DefineAll() / defineCustomElement().
 */

(function() {
   "use strict";

   /**
    * _pranaDef registra um custom element cujos callbacks de ciclo de vida
    * são implementados em Go/WASM.
    *
    * @param {string}   tagName      - nome do custom element (e.g. "my-widget")
    * @param {Function} ctor         - callback Go para constructor(self)
    * @param {Function} connected    - callback Go para connectedCallback(self)
    * @param {Function} attrChanged  - callback Go para attributeChangedCallback(self,name,old,new)
    * @param {Function} disconnected - callback Go para disconnectedCallback(self)
    * @param {string[]} observed     - lista de atributos observados
    * @param {boolean}  formAssociated - true → elemento form-associated: a classe
    *                   declara `static formAssociated` e o constructor chama
    *                   attachInternals(), exposto ao Go como `_internals`
    *                   (setValidity/setFormValue participam do <form> nativo)
    */
   window._pranaDef = function(tagName, ctor, connected, attrChanged, disconnected, observed, formAssociated) {
      customElements.define(
         tagName,
         class extends HTMLElement {
            static get formAssociated() {
               return !!formAssociated;
            }

            static get observedAttributes() {
               return observed || [];
            }

            constructor() {
               super();
               if (formAssociated) {
                  this._internals = this.attachInternals();
               }
               // Encaminha para o Go
               ctor(this);
            }

            connectedCallback() {
               connected(this);
            }

            attributeChangedCallback(name, oldValue, newValue) {
               attrChanged(this, name, oldValue ?? "", newValue ?? "");
            }

            disconnectedCallback() {
               disconnected(this);
            }
         }
      );
   };
})();
