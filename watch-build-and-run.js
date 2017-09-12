const path = require('path')
const child_process = require('child_process')
const fs = require('fs')
const extend = Object.assign

const resolve = f => path.join(__dirname, f)

function main() {

    let compileAndRunProcessPid = startCompileAndRunProcess()
    let changedId = 0

    fs.watch(resolve('main.go'), debounce(asyncFunc(function* () {
        let curChangedId = ++changedId

        console.log("[%d] Source file is changed.... so restart compile and run process", curChangedId)

        while (isRunning(compileAndRunProcessPid) && changedId === curChangedId) {
            try{
                process.kill(compileAndRunProcessPid, 'SIGTERM')
            } catch(e){}
            yield sleep(200)
        }

        if (changedId === curChangedId) {
            compileAndRunProcessPid = startCompileAndRunProcess()
        }
    }), 500))
}

function asyncFunc(generatorFunc) {
    return function () {
        return co.call(this, generatorFunc)
    }
}

function startCompileAndRunProcess() {
    let process = child_process.spawn(resolve('build-and-run.sh'), { stdio: 'inherit', shell: true })
    process.on('exit', function (code) {
        console.log("compile and run process exit: " + code)
    })

    return process.pid
}


function debounce(fn, timeout) {
    let timer = 0
    return function (...args) {
        let _this = this
        if (timer) {
            clearTimeout(timer)
        }

        timer = setTimeout(function () {
            fn.apply(_this, args)
        }, timeout)
    }
}

function isRunning(pid) {
    try {
        return process.kill(pid, 0)
    }
    catch (e) {
        return e.code === 'EPERM'
    }
}


function sleep(time) {
    return new Promise(resolve => setTimeout(() => resolve(), time))
}

const co = (function () {
    /**
     * slice() reference.
     */

    var slice = Array.prototype.slice;


    /**
     * Execute the generator function or a generator
     * and return a promise.
     *
     * @param {Function} fn
     * @return {Promise}
     * @api public
     */

    function co(gen) {
        var ctx = this;
        var args = slice.call(arguments, 1);

        // we wrap everything in a promise to avoid promise chaining,
        // which leads to memory leak errors.
        // see https://github.com/tj/co/issues/180
        return new Promise(function (resolve, reject) {
            if (typeof gen === 'function') gen = gen.apply(ctx, args);
            if (!gen || typeof gen.next !== 'function') return resolve(gen);

            onFulfilled();

            /**
             * @param {Mixed} res
             * @return {Promise}
             * @api private
             */

            function onFulfilled(res) {
                var ret;
                try {
                    ret = gen.next(res);
                } catch (e) {
                    return reject(e);
                }
                next(ret);
                return null;
            }

            /**
             * @param {Error} err
             * @return {Promise}
             * @api private
             */

            function onRejected(err) {
                var ret;
                try {
                    ret = gen.throw(err);
                } catch (e) {
                    return reject(e);
                }
                next(ret);
            }

            /**
             * Get the next value in the generator,
             * return a promise.
             *
             * @param {Object} ret
             * @return {Promise}
             * @api private
             */

            function next(ret) {
                if (ret.done) return resolve(ret.value);
                var value = toPromise.call(ctx, ret.value);
                if (value && isPromise(value)) return value.then(onFulfilled, onRejected);
                return onRejected(new TypeError('You may only yield a function, promise, generator, array, or object, '
                    + 'but the following object was passed: "' + String(ret.value) + '"'));
            }
        });
    }

    /**
     * Convert a `yield`ed value into a promise.
     *
     * @param {Mixed} obj
     * @return {Promise}
     * @api private
     */

    function toPromise(obj) {
        if (!obj) return obj;
        if (isPromise(obj)) return obj;
        if (isGeneratorFunction(obj) || isGenerator(obj)) return co.call(this, obj);
        if ('function' == typeof obj) return thunkToPromise.call(this, obj);
        if (Array.isArray(obj)) return arrayToPromise.call(this, obj);
        if (isObject(obj)) return objectToPromise.call(this, obj);
        return obj;
    }

    /**
     * Convert a thunk to a promise.
     *
     * @param {Function}
     * @return {Promise}
     * @api private
     */

    function thunkToPromise(fn) {
        var ctx = this;
        return new Promise(function (resolve, reject) {
            fn.call(ctx, function (err, res) {
                if (err) return reject(err);
                if (arguments.length > 2) res = slice.call(arguments, 1);
                resolve(res);
            });
        });
    }

    /**
     * Convert an array of "yieldables" to a promise.
     * Uses `Promise.all()` internally.
     *
     * @param {Array} obj
     * @return {Promise}
     * @api private
     */

    function arrayToPromise(obj) {
        return Promise.all(obj.map(toPromise, this));
    }

    /**
     * Convert an object of "yieldables" to a promise.
     * Uses `Promise.all()` internally.
     *
     * @param {Object} obj
     * @return {Promise}
     * @api private
     */

    function objectToPromise(obj) {
        var results = new obj.constructor();
        var keys = Object.keys(obj);
        var promises = [];
        for (var i = 0; i < keys.length; i++) {
            var key = keys[i];
            var promise = toPromise.call(this, obj[key]);
            if (promise && isPromise(promise)) defer(promise, key);
            else results[key] = obj[key];
        }
        return Promise.all(promises).then(function () {
            return results;
        });

        function defer(promise, key) {
            // predefine the key in the result
            results[key] = undefined;
            promises.push(promise.then(function (res) {
                results[key] = res;
            }));
        }
    }

    /**
     * Check if `obj` is a promise.
     *
     * @param {Object} obj
     * @return {Boolean}
     * @api private
     */

    function isPromise(obj) {
        return 'function' == typeof obj.then;
    }

    /**
     * Check if `obj` is a generator.
     *
     * @param {Mixed} obj
     * @return {Boolean}
     * @api private
     */

    function isGenerator(obj) {
        return 'function' == typeof obj.next && 'function' == typeof obj.throw;
    }

    /**
     * Check if `obj` is a generator function.
     *
     * @param {Mixed} obj
     * @return {Boolean}
     * @api private
     */

    function isGeneratorFunction(obj) {
        var constructor = obj.constructor;
        if (!constructor) return false;
        if ('GeneratorFunction' === constructor.name || 'GeneratorFunction' === constructor.displayName) return true;
        return isGenerator(constructor.prototype);
    }

    /**
     * Check for plain object.
     *
     * @param {Mixed} val
     * @return {Boolean}
     * @api private
     */

    function isObject(val) {
        return Object == val.constructor;
    }

    return co
})()


main()

